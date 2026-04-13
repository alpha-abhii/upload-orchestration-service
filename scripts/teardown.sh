#!/bin/bash
set -e

echo "🔴 Tearing down upload-orchestration-service..."

REGION="ap-south-1"
CLUSTER="upload-orchestration-cluster"
SERVICE="upload-orchestration-service"
ALB_ARN="arn:aws:elasticloadbalancing:ap-south-1:058264490348:loadbalancer/app/upload-orchestration-alb/0a698e6637e19b7b"
TG_ARN="arn:aws:elasticloadbalancing:ap-south-1:058264490348:targetgroup/upload-orchestration-tg/85dc982d3b7af976"
LISTENER_ARN="arn:aws:elasticloadbalancing:ap-south-1:058264490348:listener/app/upload-orchestration-alb/0a698e6637e19b7b/19e8d07d0f56cb6d"

# Scale down ECS service to 0
echo "⏹  Scaling down ECS service..."
aws ecs update-service \
  --cluster $CLUSTER \
  --service $SERVICE \
  --desired-count 0 \
  --region $REGION > /dev/null
echo "✅ ECS service scaled to 0"

# Delete ALB listener
echo "🗑  Deleting ALB listener..."
aws elbv2 delete-listener \
  --listener-arn $LISTENER_ARN \
  --region $REGION
echo "✅ ALB listener deleted"

# Delete ALB
echo "🗑  Deleting ALB..."
aws elbv2 delete-load-balancer \
  --load-balancer-arn $ALB_ARN \
  --region $REGION
echo "✅ ALB deleted"

# Delete target group
echo "🗑  Deleting target group..."
sleep 5
aws elbv2 delete-target-group \
  --target-group-arn $TG_ARN \
  --region $REGION
echo "✅ Target group deleted"

echo ""
echo "💰 Billing stopped for:"
echo "   - ECS Fargate tasks (scaled to 0)"
echo "   - ALB (deleted)"
echo ""
echo "💾 Still running (minimal/free cost):"
echo "   - ECR images (~\$0.10/GB/month)"
echo "   - S3 bucket (pay per GB stored)"
echo "   - SQS queue (free tier)"
echo "   - CloudWatch logs (free tier)"
echo ""
echo "▶  To redeploy: ./scripts/deploy.sh"