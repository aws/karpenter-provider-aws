# Who is using Karpenter?
Karpenter has a variety of users and use cases for scaling Kubernetes.
Many customers want to learn from others who have already implemented Karpenter in their environments.

The following is a self-reported list of users to help identify adoption and points of contact.

## Community
If you would like to ask question from the community please join the [Karpenter slack channel in the Kubernetes Slack](https://kubernetes.slack.com/archives/C02SFFZSA2K) or join the [Karpenter working group](https://karpenter.sh/docs/contributing/working-group/) bi-weekly calls.

## Add yourself
If you are using Karpenter please consider adding yourself as a user by opening a pull request to this file.
If you are open to others contacting you about your use of Karpenter on Slack, add your Slack name as well.

## Adopters (Alphabetical)

| Organization | Description | Contacts | Link |
| --- | --- | --- | --- |
| Amazon, Inc. | Scaling production workloads and batch jobs in all AWS regions | `@Alex Kestner`, `@Ellis Tarn` | [Introducing Karpenter](https://aws.amazon.com/blogs/aws/introducing-karpenter-an-open-source-high-performance-kubernetes-cluster-autoscaler/) |
| Airtel Digital Ltd. | Karpenter for all kind of spiky and base workload and to increase spot coverage. | `@Sagar Arora` | [Homepage](https://www.wynk.in)
| Anthropic | Better utilizing mixes of instance types for more reliable capacity | N/A | [Homepage](https://anthropic.com) |
| AppsFlyer | Managing statefull workloads like Kafka and more | `@alicvsroxas` `@danielvrog`| [Homepage](https://www.appsflyer.com/)|
| Astradot | Using Karpenter on all K8s clusters in production and staging | N/A | [Homepage](https://astradot.com) |
| Beeswax | Using Karpenter to scale our high load AdTech platform efficiently | `@James Wojewoda` | [Homepage](https://www.beeswax.com)
| Canva | Using Karpetner to scale CPU and GPU workloads on EKS | `@groodt` | [Canva](https://www.canva.com/) |
| Cloud Posse, LLC | Karpenter ships out-of-the-box in our Terraform Blueprint for EKS and is offered as part of our comprehensive multi-account [AWS reference architecture](https://cloudposse.com/reference-architecture/). Everything is Open Source (APACHE2). | `@osterman` | [Karpenter : The Cloud Posse Developer Hub](https://docs.cloudposse.com/components/catalog/aws/eks/karpenter/) |
| Codefresh | Juggling workloads for the SAAS CD/GitOps offering | `@Yonatan Koren`, `@Ilia Medvedev` | [Codefresh](https://codefresh.io/) |
| Conveyor | Using karpenter to scale our customers data pipelines on EKS | `@stijndehaes` | [Conveyor](https://conveyordata.com/) |
| Cordial   | Using Karpenter to scale multiple EKS clusters quickly | `@dschaaff` | [Cordial](https://cordial.com) |
| Dig Security | Protecting our customers data - Using Karpenter to manage production and development workloads on EKS, We are using only Spot Instances in production. | `@Shahar Danus` | [Dig Security](https://dig.security/) |
| Docker | Using Karpenter to scale Docker Hub on our EKS clusters | N/A | [Docker](https://www.docker.com) |
| Grafana Labs | Using Karpenter as our Autoscaling tool on EKS | `@paulajulve`, `@logyball` | [Homepage](https://grafana.com/) & [Blog](https://grafana.com/blog/2023/11/09/how-grafana-labs-switched-to-karpenter-to-reduce-costs-and-complexities-in-amazon-eks/) |
| H2O.ai | Dynamically scaling CPU and GPU nodes for AI workloads  | `@Ophir Zahavi`, `@Asaf Oren` | [H2O.ai](https://h2o.ai/) |
| idealo | Scaling multi-arch IPv6 clusters hosting web and event-driven applications | `@Heiko Rothe` | [Homepage](https://www.idealo.de) |
| Livspace | Replacement for cluster autoscaler on production and staging EKS clusters | `@praveen-livspace` | [Homepage](https://www.livspace.com) |
| Nexxiot | Easier, Safer, Cleaner Global Transportation - Using Karpenter to manage EKS nodes | `@Alex Berger` | [Homepage](https://nexxiot.com/) |
| Nirvana Money | Building healthy, happy financial lives - Using Karpenter to manage all-Spot clusters | `@DWSR` | [Homepage](https://www.nirvana.money/) |
| OccMundial | Using karpenter to manage our production workloads, using the dynamic scaling according to workload that needs to scale.  | `@parraletz` | [Homepage](https://www.occ.com.mx) |
| Omaze | Intelligently using Karpenter's autoscaling to power our platforms | `@devopsidiot` | [Homepage](https://www.omaze.com/) |
| PITS Global Data Recovery Services | Used to manage continuous integration and continuous delivery/deployment workflows. | N/A | [PITS Global Data Recovery Services](https://www.pitsdatarecovery.net/) |
| PlanetScale | Leveraging Karpenter to dynamically deploy serverless MySQL workloads. | `@jtcunning` | [Homepage](https://www.planetscale.com/) |
| QuestDB | Using Karpenter for the service nodes of the QuestBD Cloud (time-series database). | [questdb slack group](https://slack.questdb.io/) | [QuestDB](https://questdb.io/) |
| Rapid7 | Using Karpenter across all of our Kubernetes infrastructure for efficient autoscaling, both in terms of speed and cost | `@arobinson`, `@Ross Kirk`, `@Ryan Williams` | [Homepage](https://www.rapid7.com/) |
| Sendcloud | Using Karpenter to scale our k8s clusters for Europe’s #1 shipping automation platform  | N/A | [Homepage](https://www.sendcloud.com/) |
| Sentra | Using Karpenter to scale our EKS clusters, running our platform and workflows while maximizing cost-efficiency with minimal operational overhead | `@Roei Jacobovich` | [Homepage](https://sentra.io/) |
| Stone Pagamentos | Using Karpenter to do smart sizing of our clusters  | `@fabiano-amaral` | [Stone Pagamentos](https://www.stone.com.br/) |
| Stytch | Powering the scaling needs of Stytch's authentication and user-management APIs  | `@Elijah Chanakira`, `@Ovadia Harary` | [Homepage](https://www.stytch.com/) |
| Superbexperience | Using Karpenter to scale the k8s clusters running our Guest Experience Management platform  | `@Wernich Bekker` | [Homepage](https://www.superbexperience.com/) |
| Target Australia | Using Karpenter to manage (scale, auto update etc) the containerised (EKS) compute that powers much of the Target Online platform | `@gazal-k` | [Target Australia Online Shopping](https://www.target.com.au) |
| The Scale Factory | Using Karpenter (controllers on EC2/Fargate) to efficiently scale K8s workloads and empowering our customer teams to do the same  | `@marko` | [Homepage](https://www.scalefactory.com) |
| Tyk Cloud | Scaling workloads for the Cloud Free plan  | `@Artem Hluvchynskyi`, `@gowtham` | [Tyk Cloud](https://tyk.io/cloud/) |
| Wehkamp | Using Karpenter to scale the EKS clusters for our e-commerce platforms | `@ChrisV` | [Wehkamp](https://www.wehkamp.nl) & [Wehkamp Techblog](https://medium.com/wehkamp-techblog)|
| Next Insurance | Using Karpenter to manage the nodes in all our EKS clusters, including dev and prod, on demand and spots | `@moshebs` | [Homepage](https://www.nextinsurance.com)|
| Grover Group GmbH | We use Karpenter for efficient and cost effective scaling of our nodes in all of our EKS clusters | `@suraj2410` | [Homepage](https://www.grover.com/de-en) & [Engineering Techblog](https://engineering.grover.com)|
| Logz.io | Using Karpenter in all of our EKS clusters for efficient and cost effective scaling of all our K8s workloads | `@pincher95`, `@Samplify` | [Homepage](https://logz.io/)|
