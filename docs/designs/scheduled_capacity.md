# Scheduled Capacity Design
## Introduction
Today, some Kubernetes users handle their workloads by scaling up and down in a recurring pattern. These patterns are 
often indicative of some change in operational load and can come in the form of anything from a series of complex 
scaling decisions to a one-off scale decision. 

## User Stories
* As a user I can periodically scale up and scale down my resources
* As a user I can schedule a special one-off scale request for my resources
* As a user I can utilize this metric in combination with others to schedule complex scaling decisions
* As a user I can see the current and future recommended states of my resources

## Background
The important parts of Karpenter to take note of will be the HorizontalAutoscaler and the MetricsProducer. For any 
user-specified resource, the MetricsProducer will be responsible for parsing the user input, calculating the metric 
recommendation, and exposing it to the metrics endpoint. The HorizontalAutoscaler will be responsible for sending the 
signals to scale the resource by using a `promql` query to grab the metric that the MetricsProducer has created.

The core of each MetricsProducer is a reconcile loop, which runs at a pre-configured interval of time, and a record 
function.  The reconciliation ensures the metric is always being calculated, while the record function makes the data 
available to the Prometheus server at every iteration of the loop.

![](../images/scheduled-capacity-dataflow-diagram.png)

While a HorizontalAutoscaler can only scale one resource, the metric that a MetricsProducer makes available can be used 
by any amount of HorizontalAutoscalers. In addition, with a more complex `promql` 
[query](https://prometheus.io/docs/prometheus/latest/querying/basics/), a user can also use a HorizontalAutoscaler to 
scale based off multiple MetricsProducers. 

For more details, refer to [Karpenter’s design doc](DESIGN.md).

## Design
This design encompasses the `ScheduleSpec` and `ScheduledCapacityStatus` structs. The spec corresponds to the user 
input specifying the scheduled behaviors. The status will be used as a way for the user to check the state of the 
metric through `kubectl` commands. 

### Metrics Producer Spec
The `ScheduleSpec` is where the user will specify the times in which a schedule will activate and recommend what the 
value of the metric should be.

```go
type Timezone string

type ScheduleSpec struct {
   // Behaviors may be layered to achieve complex scheduling autoscaling logic
   Behaviors       []ScheduledBehavior `json:"behaviors"`
   // Defaults to UTC. Users will specify their schedules assuming this is their timezone
   // ref: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
   // +optional
   Timezone        *Timezone    `json:"timezone,omitempty"`
   // A schedule defaults to this value when no behaviors are active
   DefaultReplicas int32      `json:"defaultReplicas"`
}

// ScheduledBehavior sets the metric to a replica value based on a start and end pattern.
type ScheduledBehavior struct {
   // The value the MetricsProducer will emit when the current time is within start and end
   Replicas  int32     `json:"replicas"`
   Start     *Pattern  `json:"start"`
   End       *Pattern  `json:"end"`
}

// Pattern is a strongly-typed version of crontabs
type Pattern struct {
   // When minutes or hours are left out, they are assumed to match to 0   
   Minutes *string `json:"minutes,omitempty"`   
   Hours   *string `json:"hours,omitempty"`   
   // When Days, Months, or Weekdays are left out, 
   // they are represented by wildcards, meaning any time matches   
   Days *string `json:"days,omitempty"`   
   // List of 3-letter abbreviations i.e. Jan, Feb, Mar
   Months *string `json:"months,omitempty"`   
   // List of 3-letter abbreviations i.e. "Mon, Tue, Wed"   
   Weekdays *string `json:"weekdays,omitempty"`
}
```

The spec below details how a user might configure their scheduled behaviors. The picture to the right corresponds to 
the configuration.

This configuration is scaling up for 9-5 on weekdays (red), scaling down a little at night (green), and then scaling 
down almost fully for the weekends (blue).
![](../images/scheduled-capacity-example-schedule-graphic.png)
```yaml
apiVersion: autoscaling.karpenter.sh/v1alpha1
kind: MetricsProducer
metadata:
  name: scheduling
spec:
  schedule:
    timezone: America/Los_Angeles
    defaultReplicas: 2
    behaviors:
      // Scale way down on Friday evening for the weekend
      - replicas: 1
        start:
          weekdays: Fri
          hours: 17
        end:
          weekdays: Mon
          hours: 9
      // Scale up on Weekdays for usual traffic
      - replicas: 3
        start: 
          weekdays: Mon,Tue,Wed,Thu,Fri
          hours: 9
        end:
          weekdays: Mon,Tue,Wed,Thu,Fri
          hours: 17
      // Scale down on weekday evenings but not as much as on weekends
      - replicas: 2
        start: 
          weekdays: Mon,Tue,Wed,Thu,Fri
          hours: 17
        end:
          weekdays: Mon,Tue,Wed,Thu,Fri
          hours: 9
```

### Metrics Producer Status Struct
The `ScheduledCapacityStatus` can be used to monitor the MetricsProducer. The results of the algorithm will populate 
this struct at every iteration of the reconcile loop. A user can see the values of this struct with 
`kubectl get metricsproducers -oyaml`.
```go
type ScheduledCapacityStatus struct {
   // The current recommendation - the metric the MetricsProducer is emitting
   CurrentValue   *int32             `json:"currentValue,omitempty"` 

   // The time where CurrentValue will switch to NextValue
   NextValueTime  *apis.VolatileTime `json:"nextValueTime,omitempty"`  
   
   // The next recommendation for the metric
   NextValue      *int32             `json:"nextValue,omitempty"`  
}
```

## Algorithm Design
The algorithm will parse all behaviors and the start and end schedule formats. We find the `nextStartTime` and 
`nextEndTime` for each of the schedules. These will be the times they next match in the future. 

We say a schedule matches if the following are all true:

* The current time is before or equal to the `nextEndTime` 
* The `nextStartTime` is after or equal to the `nextEndTime` 

Based on how many schedules match:

* If there is no match, we set the metric to the `defaultReplicas`
* If there is only one match, we set the metric to that behavior’s value
* If there are multiple matches, we set the metric to the value that is specified first in the spec

This algorithm and API choice are very similar to [KEDA’s Cron Scaler](https://keda.sh/docs/2.0/scalers/cron/).

## Strongly-Typed vs Crontabs
Most other time-based schedulers use Crontabs as their API. This section discusses why we chose against Crontabs and 
how the two choices are similar.

* The [Cron library](https://github.com/robfig/cron) captures too broad of a scope for our use-case. 
    * In planning critical scaling decisions, freedom can hurt more than help. One malformed scale signal can cost the 
    user a lot more money, or even scale down unexpectedly. 
    * While our implementation will use the Cron library, picking a strongly-typed API will allows us to decide which 
    portions of the library we want to allow the users to configure.
* The wide range of functionality Cron provides is sometimes misunderstood 
(e.g. [Crontab Pitfalls](#crontab-pitfalls)).
    * Adopting Crontab syntax adopts its pitfalls, which can be hard to fix in the future. 
    * If users have common problems involving Cron, it is more difficult to fix than if they were problems specific to 
    Karpenter.
* Karpenter’s metrics signals are best described as level-triggered. Crontabs were created to describe when to trigger 
Cronjobs, which is best described as edge-triggered. 
    * If a user sees Crontabs, they may assume that Karpenter is edge-triggered behind the scenes, which 
    implies certain [problems](https://hackernoon.com/level-triggering-and-reconciliation-in-kubernetes-1f17fe30333d) 
    with availability. 
    * We want our users to infer correctly what is happening behind the scenes.
    
## Field Plurality and Configuration Bloat
While Crontabs allow a user to specify **ranges** and **lists** of numbers/strings, we chose to **only** allow a **list** of 
numbers/strings. Having a start and stop configuration in the form of Crontabs can confuse the user if they use overly 
complex configurations. Reducing the scope of their choices to just a list of values can make it clearer. 

It is important to allow a user to specify multiple values to ease configuration load. While simpler cases like below 
are easier to understand, adding more Crontab aspects like skip values and ranges can be much harder to mentally parse 
at more complex levels of planning. We want to keep the tool intuitive, precise, and understandable, so that users who 
understand their workloads can easily schedule them.

```yaml
apiVersion: autoscaling.karpenter.sh/v1alpha1
kind: MetricsProducer
metadata:
  name: FakeScheduling
spec:
  schedule:
    timezone: America/Los_Angeles
    defaultReplicas: 2
    behaviors:
      // This spec WILL NOT work according to the design.
      // Scale up on Weekdays for usual traffic
      - replicas: 7
        start: 
          weekdays: Mon-Fri
          hours: 9
          months: Jan-Mar
        end:
          weekdays: Mon-Fri
          hours: 17
          months: Feb-Apr
      // This spec WILL work according to the design.
      // Scale down on weekday evenings
      - replicas: 7
        start: 
          weekdays: Mon,Tue,Wed,Thu,Fri
          hours: 9
          months: Jan,Feb,Mar
        end:
          weekdays: Mon,Tue,Wed,Thu,Fri
          hours: 17
          months: Feb,Mar,Apr
```

## FAQ
### How does this design handle collisions right now?

*  In the MVP, if a schedule ends when another starts, it will select the schedule that is starting. If more than one 
are starting/valid, then it will use the schedule that comes first in the spec.
* Look at the Out of Scope https://quip-amazon.com/zQ7mAxg0wNDC/Karpenter-Periodic-Autoscaling#ANY9CAbqSLH below for 
more details.

### How would a priority system work for collisions?

* Essentially, every schedule would have some associated Priority. If multiple schedules match to the same time, the 
one with the higher priority will win. In the event of a tie, we resort to position in the spec. Whichever schedule is 
configured first will win.

### How can I leverage this tool to work with other metrics?

* Using this metric in tandem with others is a part of the Karpenter HorizontalAutoscaler. There are many possibilities,
 and it’s possible to do so with all metrics in prometheus, as long as they return an instant vector (a singular value).
* Let’s say a user is scaling based-off a queue (a metric currently supported by Karpenter). If they’d like to keep a 
healthy minimum value regardless of the size of the queue to stay ready for an abnormally large batch of jobs, they can 
configure their HorizontalAutoscaler’s Spec.Metrics.Prometheus.Query field to be the line below.

`max(karpenter_queue_length{name="ml-training-queue"},karpenter_scheduled_capacity{name="schedules"})` 

### Is it required to use Prometheus?

* Currently, Karpenter’s design has a dependency on Prometheus. We use Prometheus to store the data that the core design 
components (MetricsProducer, HorizontalAutoscaler, ScalableNodeGroup) use to communicate with each other.

### Why Karpenter HorizontalAutoscaler and MetricsProducer? Why not use the HPA?

* Karpenter’s design details why we have a CRD called HorizontalAutoscaler, and how our MetricsProducers complement 
them. While there are a lot of similarities, there are key differences as detailed in the design 
[here](../designs/DESIGN.md#alignment-with-the-horizontal-pod-autoscaler-api).

## Out of Scope - Additional Future Features
Our current design currently does not have a robust way to handle collisions and help visualize how the metric will look 
over time. While these are important issues, their implementations will not be included in the MVP. 

### Collisions
Collisions occur when more than one schedule matches to the current time. When this happens, the MetricsProducer cannot 
emit more than one value at a time, so it must choose one value.

* When could collisions happen past user error?
    * When a user wants a special one-off scale up request
        * e.g. MyCompany normally has `x` replicas on all Fridays at 06:00 and `y` replicas on Fridays at 20:00, but 
        wants `z` replicas on Black Friday at 06:00
        * For only this Friday, the normal Friday schedule and this special one-off request will conflict
* Solutions for collision handling
    * Create a warning with the first times that a collision could happen
        * Doesn’t decide functionality for users 
        * Does not guarantee it will be resolved
    * Associate each schedule with a priority which will be used in comparison to other colliding schedules
        * Requires users to rank each of their schedules, which they may want to change based on the time they collide
        * Ties in priority
            * Use the order in which they’re specified in the spec **OR**
            * Default to the defaultReplicas

The only change to the structs from the initial design would be to add a Priority field in the ScheduledBehavior struct 
as below.
```go
type ScheduledBehavior struct {
   Replicas  int32     `json:"replicas"`
   Start     *Pattern  `json:"start"`
   End       *Pattern  `json:"end"`
   Priority  *int32    `json:"priority,omitempty"`
}
```

### Configuration Complexity
When a user is configuring their resources, it’s easy to lose track of how the metric will look over time, especially 
if a user may want to plan far into the future with many complex behaviors. Creating a tool to visualize schedules will 
not only help users understand how their schedules will match up, but can ease concerns during the configuration 
process. This can empower users to create even more complex schedules to match their needs.

Possible Designs:

* A dual standalone tool/UI that users can use to either validate or create their YAML
    * Pros
        * Allows check-as-you-go for configuration purposes
        * Auto creates their YAML with a recommendation based
    * Cons
        * Requires users to manually use it
        * Requires a lot more work to implement a UI to help create the YAML as opposed to just a tool to validate
* An extra function to be included as part of the MetricsProducerStatus 
    * Pros
        * Always available to see the visualization with kubectl commands
    * Cons
        * Will use some compute power to keep running (may be trivial amount of compute)
        * Cannot use to check-as-you-go for configuration purposes

## Crontab Pitfalls
This design includes the choice to use a strongly-typed API due to cases where Crontabs do not act as expected. Below 
is the most common misunderstanding.

* Let's say I have a schedule to trigger on the following dates: 
    * Schedule A: First 3 Thursdays of January
    * Schedule B: The Friday of the last week of January and the first 2 weeks of February
    * Schedule C: Tuesday for every week until the end of March after Schedule B
* This is how someone might do it
    * Schedule A -  "* * 1-21 1 4" for the first three Thursdays of January
    * Schedule B - "* * 22-31 1 5" for the last week of January and "* * 1-14 2 5" for the first two weeks of February 
    * Schedule C - "* * 15-31 2 2" for the last Tuesdays in February and "* * * 3 2" for the Tuesdays in March
* Problems with the above approach
    * Schedule A will match to any day in January that is in 1/1 to 1/21 or is a Thursday
    * Schedule B’s first crontab will match to any day in January that is in 1/22 to 1/31 or is a Friday
    * Schedule B’s second crontab will match to any day in February that is in 2/1 to 2/14 or is a Friday
    * Schedule C’s first crontab will match to any day in February that is in 2/15 to 2/31 or is a Tuesday
    * Schedule C’s second crontab is the only one that works as intended.
* The way that crontabs are implemented is if both Dom and Dow are non-wildcards (as they are above in each of the 
crontabs except for Schedule C’s second crontab), then the crontab is treated as a match if **either** the Dom **or** Dow 
matches. 





