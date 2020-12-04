# Working Group
Karpenter's community is open to everyone. All invites are managed through our [Calendar](https://calendar.google.com/calendar/u/0?cid=N3FmZGVvZjVoZWJkZjZpMnJrMmplZzVqYmtAZ3JvdXAuY2FsZW5kYXIuZ29vZ2xlLmNvbQ). Alternatively, you can use our [iCal Export](https://calendar.google.com/calendar/ical/7qfdeof5hebdf6i2rk2jeg5jbk%40group.calendar.google.com/public/basic.ics) to add the events to Outlook or other email providers.


# Notes
Please contribute to our meeting notes by opening a PR.

## Template
1. Community Questions
2. Work Items
3. Demos

# Meeting Notes (12/4/2020)
## Attendees
@ellistarn
@prateekgogia
@gjtempleton
@shreyas87

## Notes:
-  [Ellis] Shared background
-  [Guy] Cloudwatch metrics, ECS scaling using cloudwatch metrics for autoscaling.
-  [Guy] Karpenter supporting generic cloudwatch metrics?
-  [Guy] Node autoscaling is supported?
-  [Ellis] Cloud provider like model for cloudwatch, provider model exists in scalable node group side.
-  [Ellis] Cloudwatch could support Prometheus API?
-  [Ellis] We can have a direct cloudwatch integration and later refine it?
-  [Guy] Implementing a generic cloud provider in core in CA.
-  [Ellis]  Will explore integration with cloudwatch directly, prefered will be coud provider model.
-  [Guy] Contributions- People in squad will be interested, open to contribute features if it provides value to the team.
-  [Guy] Scaling on non-pending pods and other resources, people have been asking. Karpenter looks promising for these aspects.
-  [Ellis] - Long term goal, upstream project as an alternative. As open as possible and vendor neutral.
-  [Guy] - There is a space for an alternative, given the history CA works around pending pods. Wider adoption possible if mature.
-  [Ellis] - Landing point will be sig-autoscaling.
-  [Guy] - CA lacks cron scheduling scaling.
-  [Ellis] - pending pods are a big requirements.
-  [Prateek] - introduced the pending pods producer proposal.
-  [Ellis] - Move time earlier by an hour and change day to Thursday, create a GH issue to get feedback what time works?