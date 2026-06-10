package cmd

import (
	"github.com/spf13/cobra"

	"k3c/cluster"
)

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Manage cluster kubeconfigs",
}

var kubeconfigGetCmd = &cobra.Command{
	Use:   "get [NAME]",
	Short: "Print the cluster's kubeconfig to stdout",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fail(cluster.KubeconfigGet(loadConfig(args)))
	},
}

var kubeconfigMergeCmd = &cobra.Command{
	Use:     "merge [NAME]",
	Aliases: []string{"write"},
	Short:   "Merge into ~/.kube/config and switch the current context",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fail(cluster.KubeconfigMerge(loadConfig(args)))
	},
}

func init() {
	kubeconfigCmd.AddCommand(kubeconfigGetCmd, kubeconfigMergeCmd)
	rootCmd.AddCommand(kubeconfigCmd)
}
