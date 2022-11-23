package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"helm-issue/internal/k8s"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		k8s1, err := k8s.NewK8s(ctx, "k8s1", "eu-west-3", &k8s.K8sArgs{
			Region: pulumi.String("eu-west-3"),
		})
		if err != nil {
			return err
		}

		k8s2, err := k8s.NewK8s(ctx, "k8s2", "eu-central-1", &k8s.K8sArgs{
			Region: pulumi.String("eu-central-1"),
		})
		if err != nil {
			return err
		}

		ctx.Export("kubeconfig-1", pulumi.ToSecret(k8s1.Kubeconfig))
		ctx.Export("kubeconfig-2", pulumi.ToSecret(k8s2.Kubeconfig))

		return nil
	})
}
