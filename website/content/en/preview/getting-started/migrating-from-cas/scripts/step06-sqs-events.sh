QUEUE_ARN=$(aws sqs get-queue-attributes --queue-url "$(aws sqs get-queue-url --queue-name "${CLUSTER_NAME}" --query 'QueueUrl' --output text)" --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

# Create EventBridge rules for EC2 interruption events
for RULE_NAME in "ScheduledChangeRule" "SpotInterruptionRule" "RebalanceRule" "InstanceStateChangeRule"; do
    case $RULE_NAME in
        ScheduledChangeRule)
            PATTERN='{"source":["aws.health"],"detail-type":["AWS Health Event"]}'
            ;;
        SpotInterruptionRule)
            PATTERN='{"source":["aws.ec2"],"detail-type":["EC2 Spot Instance Interruption Warning"]}'
            ;;
        RebalanceRule)
            PATTERN='{"source":["aws.ec2"],"detail-type":["EC2 Instance Rebalance Recommendation"]}'
            ;;
        InstanceStateChangeRule)
            PATTERN='{"source":["aws.ec2"],"detail-type":["EC2 Instance State-change Notification"]}'
            ;;
    esac

    aws events put-rule \
        --name "KarpenterInterruptionRule-${CLUSTER_NAME}-${RULE_NAME}" \
        --event-pattern "${PATTERN}"

    aws events put-targets \
        --rule "KarpenterInterruptionRule-${CLUSTER_NAME}-${RULE_NAME}" \
        --targets "Id=KarpenterInterruptionQueueTarget,Arn=${QUEUE_ARN}"
done
