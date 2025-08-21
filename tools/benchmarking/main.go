package main

import (
	"benchmark/bench"
	instanceconfig "benchmark/config"
	"benchmark/metrics"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/yaml"
)

var (
	kubeconfig *string
	yamlFile   *string
	namespace  *string
	replicas   *int64
	benchName  *string
)

type CostDataPoint struct {
	Timestamp time.Duration
	Cost      float64
}

func main() {
	home := homedir.HomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")

	yamlFile = flag.String("file", "", "path to the deployment YAML file")
	namespace = flag.String("namespace", "default", "namespace for the deployment")
	replicas = flag.Int64("replicas", 100, "number of replicas to scale to")
	benchName = flag.String("bench", "", "name of the benchmark to run")
	instanceTypesFile := flag.String("instance-types", "", "path to the instance types JSON file")
	flag.Parse()

	if *benchName == "" || *yamlFile == "" || *instanceTypesFile == "" {
		log.Fatalf("Benchmark name, deployment yaml, and instance types are required")
	}

	// Build REST config from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	// Create controller-runtime client
	kubeClient := lo.Must(client.New(config, client.Options{}))

	ctx := context.Background()

	// Validate Karpenter is Ready and running since it is restarted between tests
	// Check if the Karpenter deployment is ready
	rollKarpenterDeployment()

	// Check if Karpenter deployment is ready
	if err := wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
		var deployment appsv1.Deployment
		if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: "kube-system", Name: "karpenter"}, &deployment); err != nil {
			return false, nil
		}
		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentAvailable && condition.Status == v1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		log.Fatalf("Karpenter deployment is not ready: %v", err)
	}

	if *yamlFile == "" {
		log.Fatalf("--file flag is required when creating a new deployment")
	}

	yamlData, err := os.ReadFile(*yamlFile)
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	// Parse the YAML file into a Deployment object
	var newDeployment appsv1.Deployment
	if err := yaml.Unmarshal(yamlData, &newDeployment); err != nil {
		log.Fatalf("Error parsing YAML file: %v", err)
	}

	// Use the namespace from the flag if not specified in the YAML
	if newDeployment.Namespace == "" {
		newDeployment.Namespace = *namespace
	} else {
		// Update the namespace variable to match what's in the YAML
		*namespace = newDeployment.Namespace
	}

	var costMap map[string]float64

	// Load instance types from file if provided
	if *instanceTypesFile != "" {
		var err error
		costMap, err = instanceconfig.LoadInstanceTypesFromFile(*instanceTypesFile)
		if err != nil {
			log.Fatalf("Failed loading instance types from file: %v", err)
		}
	}

	// Create the deployment
	fmt.Printf("Creating deployment %s...\n", newDeployment.Name)

	// Check if deployment already exists
	var existingDeployment appsv1.Deployment
	err = kubeClient.Get(ctx, types.NamespacedName{Namespace: newDeployment.Namespace, Name: newDeployment.Name}, &existingDeployment)

	var deployment *appsv1.Deployment
	if err == nil {
		// Deployment exists
		log.Printf("Deployment %s already exists in namespace. Using existing deployment.", newDeployment.Name)
		deployment = &existingDeployment
	} else if errors.IsNotFound(err) {
		// Deployment doesn't exist, create it
		if err := kubeClient.Create(ctx, &newDeployment); err != nil {
			log.Fatalf("Error creating deployment: %v", err)
		}
		deployment = &newDeployment
	} else {
		log.Fatalf("Error checking if deployment exists: %v", err)
	}

	// Ensure cleanup on panic or early exit
	defer func() {
		fmt.Println("Cleaning up resources...")
		cleanupDeployment(ctx, kubeClient, deployment.Namespace, deployment.Name)
		cleanupNodes(ctx, kubeClient)
	}()

	// Scale the deployment to target replicas
	fmt.Printf("Scaling deployment to %d replicas...\n", *replicas)
	scaleStartTime := time.Now()

	// Get the deployment
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, deployment); err != nil {
		log.Fatalf("Error getting deployment: %v", err)
	}

	stored := deployment.DeepCopy()
	deployment.Spec.Replicas = lo.ToPtr(int32(*replicas))

	if err := kubeClient.Patch(ctx, deployment, client.StrategicMergeFrom(stored)); err != nil {
		log.Fatalf("Error patching deployment: %v", err)
	}

	// Wait for all pods to be ready
	fmt.Println("Waiting for all pods to be ready...")
	if err = waitForDeploymentReady(ctx, kubeClient, deployment.Namespace, deployment.Name, *replicas); err != nil {
		log.Fatalf("Error waiting for deployment to be ready: %v", err)
	}

	scaleUpDuration := time.Since(scaleStartTime)
	fmt.Printf("All pods are ready! Time taken: %v\n", scaleUpDuration)

	// Get the list of nodes used by the deployment
	deploymentNodes, err := getNodesForDeployment(ctx, kubeClient, deployment.Namespace, deployment.Name)
	if err != nil {
		log.Fatalf("Error getting nodes for deployment: %v", err)
	}
	fmt.Printf("Deployment is running on %d nodes\n", len(deploymentNodes))
	totalCost := getCostForNodes(ctx, kubeClient, deploymentNodes, costMap)
	fmt.Printf("Total cost for nodes: %f\n", totalCost)

	// Select and run the appropriate test suite
	// Create test context
	def := bench.Defintion{
		Replicas: int32(*replicas),
	}

	var testSuite bench.Suite
	switch *benchName {
	case "emptiness":
		testSuite = &bench.EmptinessTestSuite{Defintion: def}
	case "consolidation":
		testSuite = &bench.ConsolidationTestSuite{Defintion: def}
	default:
		log.Fatalf("Unknown test suite: %s", *benchName)
	}

	fmt.Printf("Running test suite: %s\n", *benchName)
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, deployment); err != nil {
		log.Fatalf("Error getting deployment: %v", err)
	}

	if err = testSuite.Run(ctx, kubeClient, deployment); err != nil {
		log.Fatalf("Error running test suite: %v", err)
	}

	// for 10 minutes or until scaled all the way down, collect cost data points every five seconds
	fmt.Println("Monitoring cost for 10 minutes or until completely scaled down...")
	startTime := time.Now()

	// Create a slice to store cost data points
	var costDataPoints []CostDataPoint

	for time.Since(startTime) < 10*time.Minute {
		elapsed := time.Since(startTime)
		currentCost := getCostForNodes(ctx, kubeClient, deploymentNodes, costMap)

		// Store the data point
		costDataPoints = append(costDataPoints, CostDataPoint{
			Timestamp: elapsed.Round(time.Second),
			Cost:      currentCost,
		})

		fmt.Printf("Current cost at %v: %f\n", elapsed.Round(time.Second), currentCost)
		if currentCost == 0 {
			fmt.Println("Cost is 0, stopping...")
			break
		}
		time.Sleep(5 * time.Second)
	}

	// Print summary of collected data points
	fmt.Println("\nCost Data Summary:")
	fmt.Println("==================")
	fmt.Printf("Total data points collected: %d\n", len(costDataPoints))

	// Calculate min, max, and average costs
	minCost := costDataPoints[0].Cost
	maxCost := costDataPoints[0].Cost
	cost := 0.0

	for _, dp := range costDataPoints {
		if dp.Cost < minCost {
			minCost = dp.Cost
		}
		if dp.Cost > maxCost {
			maxCost = dp.Cost
		}
		cost += dp.Cost
	}

	avgCost := cost / float64(len(costDataPoints))

	fmt.Println("\nSummary:")
	fmt.Printf("Scale up time (0 to %d pods): %v\n", *replicas, scaleUpDuration)
	fmt.Printf("Initial cost: %f\n", costDataPoints[0].Cost)
	fmt.Printf("Final cost: %f\n", costDataPoints[len(costDataPoints)-1].Cost)
	fmt.Printf("Minimum cost: %f\n", minCost)
	fmt.Printf("Maximum cost: %f\n", maxCost)
	fmt.Printf("Average cost: %f\n", avgCost)

	reductionPercent := (costDataPoints[0].Cost - costDataPoints[len(costDataPoints)-1].Cost) / costDataPoints[0].Cost * 100
	fmt.Printf("Cost reduction: %.2f%%\n", reductionPercent)
	if err := metrics.PrintMetrics("localhost:8080/metrics"); err != nil {
		log.Fatalf("processing metrics: %v", err)
	}
}

// waitForDeploymentReady waits for all pods in a deployment to be ready
func waitForDeploymentReady(ctx context.Context, kubeClient client.Client, namespace, name string, replicas int64) error {
	return wait.PollImmediate(1*time.Second, 60*time.Minute, func() (bool, error) {
		var deployment appsv1.Deployment
		if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
			return false, err
		}

		// List pods with the deployment's selector
		var podList v1.PodList
		if err := kubeClient.List(ctx, &podList, client.InNamespace(namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
			return false, fmt.Errorf("failed to list pods: %w", err)
		}

		if len(podList.Items) != int(replicas) {
			return false, nil
		}

		for _, pod := range podList.Items {
			if pod.Spec.NodeName == "" {
				return false, nil
			}
		}

		return true, nil
	})
}

// getNodesForDeployment returns a list of node names where the deployment pods are running
func getNodesForDeployment(ctx context.Context, kubeClient client.Client, namespace, name string) (map[string]bool, error) {
	var deployment appsv1.Deployment
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
		return nil, err
	}

	var podList v1.PodList
	if err := kubeClient.List(ctx, &podList, client.InNamespace(namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
		return nil, err
	}

	nodeMap := make(map[string]bool)
	for _, pod := range podList.Items {
		if pod.Spec.NodeName != "" {
			nodeMap[pod.Spec.NodeName] = true
		}
	}

	return nodeMap, nil
}

func cleanupDeployment(ctx context.Context, kubeClient client.Client, namespace, name string) error {
	var deployment appsv1.Deployment
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err != nil {
		return fmt.Errorf("error getting deployment for cleanup: %v", err)
	}

	// Delete the deployment
	fmt.Printf("Deleting deployment %s in namespace %s\n", name, namespace)
	if err := kubeClient.Delete(ctx, &deployment); err != nil {
		return fmt.Errorf("error deleting deployment: %v", err)
	}

	fmt.Println("Waiting for deployment to be deleted...")
	if err := wait.PollImmediate(1*time.Second, 5*time.Minute, func() (bool, error) {
		var checkDeployment appsv1.Deployment
		err := kubeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &checkDeployment)
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("error waiting for deployment deletion: %v", err)
	}

	// Check for any remaining pods with the deployment's labels
	fmt.Println("Checking for any remaining pods...")
	if err := wait.PollImmediate(1*time.Second, 5*time.Minute, func() (bool, error) {
		var podList v1.PodList
		if err := kubeClient.List(ctx, &podList, client.InNamespace(namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
			return false, err
		}

		if len(podList.Items) > 0 {
			fmt.Printf("Found %d pods still remaining, waiting...\n", len(podList.Items))
			return false, nil
		}
		return true, nil
	}); err != nil {
		return fmt.Errorf("error waiting for pods to be deleted: %v", err)
	}

	fmt.Println("All deployment resources have been successfully cleaned up.")
	return nil
}

func rollKarpenterDeployment() {
	cmd := exec.Command("kubectl", "-n", "kube-system", "rollout", "restart", "deployment", "karpenter")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Error restarting Karpenter deployment: %v", err)
	}
}

func getCostForNodes(ctx context.Context, kubeClient client.Client, deploymentNodes map[string]bool, costMap map[string]float64) float64 {
	var nodeList v1.NodeList
	if err := kubeClient.List(ctx, &nodeList); err != nil {
		log.Printf("Error listing nodes: %v", err)
		return 0.0
	}

	totalCost := 0.0
	for _, node := range nodeList.Items {
		if deploymentNodes[node.Name] {
			totalCost += costMap[node.Labels["node.kubernetes.io/instance-type"]]
		}
	}

	return totalCost
}

func cleanupNodes(ctx context.Context, kubeClient client.Client) {
	nodeList := &v1.NodeList{}
	nodeClaimList := &karpv1.NodeClaimList{}
	sem := make(chan struct{}, 10) // 10 concurrent requests
	wg := sync.WaitGroup{}
	// poll for 5 minutes
	if err := wait.PollImmediate(1*time.Second, 5*time.Minute, func() (bool, error) {
		if err := kubeClient.List(ctx, nodeClaimList); err != nil {
			return false, err
		}
		if err := kubeClient.List(ctx, nodeList); err != nil {
			return false, err
		}
		if len(nodeClaimList.Items) == 0 && len(nodeList.Items) == 0 {
			return true, nil
		}
		for _, nodeClaim := range nodeClaimList.Items {
			go func(nodeClaim karpv1.NodeClaim) {
				sem <- struct{}{} // acquire semaphore
				defer func() { <-sem }()
				wg.Add(1)
				if err := kubeClient.Delete(ctx, &nodeClaim); err != nil {
					log.Printf("Error deleting node claim %s: %v", nodeClaim.Name, err)
				}
			}(nodeClaim)
		}
		wg.Wait()

		for _, node := range nodeList.Items {
			go func(node v1.Node) {
				sem <- struct{}{} // acquire semaphore
				defer func() { <-sem }()
				wg.Add(1)
				if err := kubeClient.Delete(ctx, &node); err != nil {
					log.Printf("Error deleting node %s: %v", node.Name, err)
				}
			}(node)
		}
		wg.Wait()

		return true, nil
	}); err != nil {
		log.Fatal("failed to clean up")
	}
}
