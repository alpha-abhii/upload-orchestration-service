#!/bin/bash
set -e

echo "🟢 Deploying upload-orchestration-service..."

REGION="ap-south-1"
CLUSTER="upload-orchestration-cluster"
SERVICE="upload-orchestration-service"
VPC_ID="vpc-01e5f5cea58742d06"
SUBNETS="subnet-025e81251013fb9dd subnet-0ee2137347bb4b7d0 subnet-0299cd7421cc9b94e"
SG_ID="sg-0882328435cdc0a54"
ACCOUNT_ID="058264490348"
ECR_REPO="$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com/upload-orchestration-service"
SQS_QUEUE_URL="https://sqs.$REGION.amazonaws.com/$ACCOUNT_ID/upload-orchestration-events"

# Step 1: Build and push latest image
echo "🔨 Building Docker image..."
docker build -t upload-orchestration-service:latest .
docker tag upload-orchestration-service:latest $ECR_REPO:latest

echo "📦 Pushing to ECR..."
aws ecr get-login-password --region $REGION | \
  docker login --username AWS --password-stdin \
  $ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com
docker push $ECR_REPO:latest
echo "✅ Image pushed to ECR"

# Step 2: Create ALB
echo "⚖️  Creating ALB..."
ALB_ARN=$(aws elbv2 create-load-balancer \
  --name upload-orchestration-alb \
  --subnets $SUBNETS \
  --security-groups $SG_ID \
  --region $REGION \
  --query "LoadBalancers[0].LoadBalancerArn" \
  --output text)
echo "✅ ALB created: $ALB_ARN"

# Step 3: Create target group
echo "🎯 Creating target group..."
TG_ARN=$(aws elbv2 create-target-group \
  --name upload-orchestration-tg \
  --protocol HTTP \
  --port 8080 \
  --vpc-id $VPC_ID \
  --target-type ip \
  --health-check-path /health \
  --health-check-interval-seconds 30 \
  --health-check-timeout-seconds 10 \
  --healthy-threshold-count 2 \
  --unhealthy-threshold-count 3 \
  --region $REGION \
  --query "TargetGroups[0].TargetGroupArn" \
  --output text)
echo "✅ Target group created: $TG_ARN"

# Step 4: Create listener
echo "👂 Creating ALB listener..."
aws elbv2 create-listener \
  --load-balancer-arn $ALB_ARN \
  --protocol HTTP \
  --port 80 \
  --default-actions Type=forward,TargetGroupArn=$TG_ARN \
  --region $REGION > /dev/null
echo "✅ Listener created"

# Step 5: Register task definition
echo "📋 Registering task definition..."
TASK_DEF_ARN=$(aws ecs register-task-definition \
  --region $REGION \
  --cli-input-json "{
    \"family\": \"upload-orchestration-service\",
    \"networkMode\": \"awsvpc\",
    \"requiresCompatibilities\": [\"FARGATE\"],
    \"cpu\": \"256\",
    \"memory\": \"512\",
    \"executionRoleArn\": \"arn:aws:iam::$ACCOUNT_ID:role/upload-orchestration-execution-role\",
    \"taskRoleArn\": \"arn:aws:iam::$ACCOUNT_ID:role/upload-orchestration-task-role\",
    \"containerDefinitions\": [{
      \"name\": \"upload-orchestration-service\",
      \"image\": \"$ECR_REPO:latest\",
      \"portMappings\": [{\"containerPort\": 8080, \"protocol\": \"tcp\"}],
      \"environment\": [
        {\"name\": \"AWS_REGION\", \"value\": \"$REGION\"},
        {\"name\": \"S3_BUCKET\", \"value\": \"upload-orchestrator-service-data\"},
        {\"name\": \"SQS_QUEUE_URL\", \"value\": \"$SQS_QUEUE_URL\"}
      ],
      \"logConfiguration\": {
        \"logDriver\": \"awslogs\",
        \"options\": {
          \"awslogs-group\": \"/ecs/upload-orchestration-service\",
          \"awslogs-region\": \"$REGION\",
          \"awslogs-stream-prefix\": \"ecs\"
        }
      },
      \"essential\": true
    }]
  }" \
  --query "taskDefinition.taskDefinitionArn" \
  --output text)
echo "✅ Task definition registered: $TASK_DEF_ARN"

# Step 6: Scale up ECS service
echo "🚀 Scaling up ECS service..."
aws ecs update-service \
  --cluster $CLUSTER \
  --service $SERVICE \
  --desired-count 1 \
  --task-definition $TASK_DEF_ARN \
  --load-balancers "targetGroupArn=$TG_ARN,containerName=upload-orchestration-service,containerPort=8080" \
  --region $REGION > /dev/null
echo "✅ ECS service scaled to 1"

# Step 7: Get ALB DNS
echo "⏳ Waiting for ALB to be active..."
sleep 30
ALB_DNS=$(aws elbv2 describe-load-balancers \
  --load-balancer-arns $ALB_ARN \
  --region $REGION \
  --query "LoadBalancers[0].DNSName" \
  --output text)

echo ""
echo "✅ Deployment complete!"
echo ""
echo "🌐 Your service URL: http://$ALB_DNS"
echo "🏥 Health check: curl http://$ALB_DNS/health"
echo ""
echo "⏹  To teardown: ./scripts/teardown.sh"