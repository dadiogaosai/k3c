package cmd

import (
	"github.com/spf13/cobra"

	"k3c/cluster"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Manage images in a cluster",
}

var imageImportCmd = &cobra.Command{
	Use:   "import IMAGE [CLUSTER]",
	Short: "Import an image from the host image store into the cluster",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		fail(cluster.ImageImport(loadConfig(args[1:]), args[0]))
	},
}

var pullCacheCmd = &cobra.Command{
	Use:   "pull-cache",
	Short: "Inspect the pull-through registry cache (shared across clusters)",
}

var pullCacheListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the cached images",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fail(cluster.PullCacheList(loadConfigDefault(nil)))
	},
}

var pullCacheInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show pull cache object count and size",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fail(cluster.PullCacheInfo(loadConfigDefault(nil)))
	},
}

var pullCacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Empty the pull cache",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fail(cluster.PullCacheClear(loadConfigDefault(nil)))
	},
}

func init() {
	pullCacheCmd.AddCommand(pullCacheListCmd, pullCacheInfoCmd, pullCacheClearCmd)
	imageCmd.AddCommand(imageImportCmd, pullCacheCmd)
	rootCmd.AddCommand(imageCmd)
}
