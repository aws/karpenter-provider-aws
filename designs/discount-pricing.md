# Discounted Pricing Support

## Overview

Karpenter is currently unaware of any discounted pricing, such as volume discounts or reserved instances/savings plans, which can lead to more expensive instances being chosen. For example an on demand instance with a savings plan discount may cost less than a spot instance of the same type. This pricing is apparently only available from the "payer" account, not any other child account API's for pricing.

This was made explicit recently - the price of spot instances rose sharply at the beginning of Q2 2023. For users on default pricing this may not be noticeable, however if there is any discount for on-demand instances in an account, it could begin to become cheaper to use on-demand instances instead of spot.

## User Stories

* Karpenter will automatically prioritise the cheapest node capacity type in an account based on personal modifications to EC2 pricing
* Karpenter will allow me to configure discounted pricing at the account level

## Background

[Conversation on Slack](https://kubernetes.slack.com/archives/C02SFFZSA2K/p1684246928553159)

## How Will Karpenter Handle Discounted Pricing

A multiplier will be applied to the price to allow any discounts to be applied and determine the real cost of an EC2 instance.
For example, a multiplier of 0.9 would apply a 10% discount

Separate multiplier values for Spot and On Demand pricing will be allowed to allow for accounts which have different pricing discounts for each type.

The multiplier will default to a value of 1 so no discount will be applied unless explicitly enabled. 

### Spot pricing

The Spot price will be multiplied with the SpotPriceMultiplier to determine the real cost for Spot instances.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: karpenter-global-settings
  namespace: karpenter
data:
  # Spot Price Multiplier for including volume discounts etc. for spot prices. The spot price will be multiplied with the spotPriceMultiplier to determine the real cost
  aws.spotPriceMultiplier: "0.95"
```

### On Demand pricing

The On Demand price will be multiplied with the OnDemandPriceMultiplier to determine the real cost for On Demand instances.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: karpenter-global-settings
  namespace: karpenter
data:
  # On Demand Multiplier for including volume discounts etc. to ensure choosing the cheapest available instance. The ondemand price will be multiplied with the onDemandPriceMultiplier to determine the real cost
  aws.onDemandPriceMultiplier: "0.60"
```
