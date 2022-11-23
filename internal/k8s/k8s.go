package k8s

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type K8s struct {
	pulumi.ResourceState

	Kubeconfig pulumi.StringOutput `pulumi:"kubeconfig"`
}

type K8sArgs struct {
	Region pulumi.StringInput `pulumi:"region"`
}

func BoolPtr(b bool) *bool {
	return &b
}

func NewK8s(ctx *pulumi.Context, name, region string, args *K8sArgs, opts ...pulumi.ResourceOption) (*K8s, error) {
	k8s := &K8s{}
	err := ctx.RegisterComponentResource("k8s:dirien:eks-k8s", name, k8s, opts...)
	if err != nil {
		return nil, err
	}
	provider, err := aws.NewProvider(ctx, fmt.Sprintf("%s-aws-provider", name), &aws.ProviderArgs{
		Region: args.Region,
	}, pulumi.Parent(k8s))
	if err != nil {
		return nil, err
	}

	vpc, err := ec2.LookupVpc(ctx, &ec2.LookupVpcArgs{
		Default: BoolPtr(true),
	}, pulumi.Parent(k8s), pulumi.Provider(provider), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	subnets, err := ec2.GetSubnets(ctx, &ec2.GetSubnetsArgs{
		Filters: []ec2.GetSubnetsFilter{
			{
				Name: "vpc-id",
				Values: []string{
					vpc.Id,
				},
			},
		},
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}
	eksRole, err := iam.NewRole(ctx, fmt.Sprintf("%s-eks-iam-role", name), &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {
						"Service": "eks.amazonaws.com"
					},
					"Action": "sts:AssumeRole"	
				}
			]
		}`),
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-eks-iam-role-attachment", name), &iam.RolePolicyAttachmentArgs{
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"),
		Role:      eksRole.Name,
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}
	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-eks-iam-role-attachment2", name), &iam.RolePolicyAttachmentArgs{
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKSServicePolicy"),
		Role:      eksRole.Name,
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	nodeRole, err := iam.NewRole(ctx, fmt.Sprintf("%s-node-iam-role", name), &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {
						"Service": "ec2.amazonaws.com"
					},
					"Action": "sts:AssumeRole"
				}
			]
		}`),
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}
	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-node-iam-role-attachment", name), &iam.RolePolicyAttachmentArgs{
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"),
		Role:      nodeRole.Name,
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}
	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-node-iam-role-attachment2", name), &iam.RolePolicyAttachmentArgs{
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"),
		Role:      nodeRole.Name,
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}
	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-node-iam-role-attachment3", name), &iam.RolePolicyAttachmentArgs{
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"),
		Role:      nodeRole.Name,
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	securityGroup, err := ec2.NewSecurityGroup(ctx, fmt.Sprintf("%s-eks-sg", name), &ec2.SecurityGroupArgs{
		Description: pulumi.String("EKS Security Group"),
		Ingress: ec2.SecurityGroupIngressArray{
			ec2.SecurityGroupIngressArgs{
				Description: pulumi.String("Allow HTTP from VPC"),
				FromPort:    pulumi.Int(80),
				Protocol:    pulumi.String("tcp"),
				ToPort:      pulumi.Int(80),
				CidrBlocks: pulumi.StringArray{
					pulumi.String("0.0.0.0/0"),
				},
			},
		},
		Egress: ec2.SecurityGroupEgressArray{
			ec2.SecurityGroupEgressArgs{
				Description: pulumi.String("Allow all outbound traffic"),
				FromPort:    pulumi.Int(0),
				Protocol:    pulumi.String("-1"),
				ToPort:      pulumi.Int(0),
				CidrBlocks: pulumi.StringArray{
					pulumi.String("0.0.0.0/0"),
				},
			},
		},
		VpcId: pulumi.String(vpc.Id),
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	eksCluster, err := eks.NewCluster(ctx, fmt.Sprintf("%s-eks-cluster", name), &eks.ClusterArgs{
		RoleArn: eksRole.Arn,
		VpcConfig: &eks.ClusterVpcConfigArgs{
			SubnetIds: toPulumiStringArray(subnets.Ids),
			PublicAccessCidrs: pulumi.StringArray{
				pulumi.String("0.0.0.0/0"),
			},
			SecurityGroupIds: pulumi.StringArray{
				securityGroup.ID(),
			},
		},
		Tags: pulumi.StringMap{
			"Name": pulumi.String(name),
		},
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	nodeGroup, err := eks.NewNodeGroup(ctx, fmt.Sprintf("%s-eks-node-group", name), &eks.NodeGroupArgs{
		ClusterName:   eksCluster.Name,
		NodeRoleArn:   nodeRole.Arn,
		SubnetIds:     toPulumiStringArray(subnets.Ids),
		NodeGroupName: pulumi.String("eks-node-group"),
		ScalingConfig: &eks.NodeGroupScalingConfigArgs{
			DesiredSize: pulumi.Int(2),
			MaxSize:     pulumi.Int(2),
			MinSize:     pulumi.Int(1),
		},
	}, pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	kubeconfig := pulumi.All(eksCluster.Name, eksCluster.Endpoint, eksCluster.CertificateAuthority).ApplyT(func(args []interface{}) (string, error) {
		name := args[0].(string)
		endpoint := args[1].(string)
		ca := args[2].(eks.ClusterCertificateAuthority)
		kubeconfig := fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
kind: Config
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws-iam-authenticator
      args:
        - "token"
        - "-i"
        - "%s"
`, *ca.Data, endpoint, name)
		return kubeconfig, nil

	}).(pulumi.StringOutput)

	k8sProvider, err := kubernetes.NewProvider(ctx, fmt.Sprintf("%s-k8sprovider", name), &kubernetes.ProviderArgs{
		Kubeconfig: kubeconfig,
	}, pulumi.DependsOn([]pulumi.Resource{nodeGroup}), pulumi.Parent(k8s), pulumi.Provider(provider))

	_, err = helm.NewChart(ctx, fmt.Sprintf("%s-helm", name), helm.ChartArgs{
		Chart:   pulumi.String("silly-helm"),
		Version: pulumi.String("0.1.4"),
		FetchArgs: helm.FetchArgs{
			Repo: pulumi.String("https://dirien.github.io/silly-helm-chart"),
		},
	}, pulumi.Provider(k8sProvider), pulumi.Parent(k8s), pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}

	k8s.Kubeconfig = kubeconfig

	err = ctx.RegisterResourceOutputs(k8s, pulumi.Map{
		"kubeconfig": kubeconfig,
	})
	if err != nil {
		return nil, err
	}

	return k8s, nil
}

func toPulumiStringArray(a []string) pulumi.StringArrayInput {
	var res []pulumi.StringInput
	for _, s := range a {
		res = append(res, pulumi.String(s))
	}
	return pulumi.StringArray(res)
}
