package main

import (
	"fmt"
	"github.com/aviator-co/av/internal/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(config.Version)
		return nil
	},
}
