// Copyright 2026 — see LICENSE file for terms.
package main

import (
	"go.pennock.tech/aifr/internal/engine"

	"github.com/spf13/cobra"
)

var (
	gitconfigScope      string
	gitconfigRegexp     string
	gitconfigSection    string
	gitconfigList       bool
	gitconfigType       string
	gitconfigStructured string
	gitconfigRepo       string
)

var gitconfigCmd = &cobra.Command{
	Use:   "git-config [key]",
	Short: "Query git configuration",
	Long: `Query git configuration values. Default scope is local (.git/config).

Scopes: local (default), merged (system+global+local+worktree with includes),
        global (~/.gitconfig), system (/etc/gitconfig)

Merged scope resolves [include] and [includeIf "gitdir:"] directives.
Unsupported conditions (onbranch, hasconfig) are reported in output.
Credential-related keys are always redacted.

Examples:
  aifr git-config remote.origin.url
  aifr git-config --scope merged user.email
  aifr git-config --regexp 'remote\..*\.url'
  aifr git-config --section remote.origin
  aifr git-config --list --scope local
  aifr git-config --identity
  aifr git-config --remotes
  aifr git-config --branches
  aifr git-config --type bool core.filemode`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		eng, err := buildEngine()
		if err != nil {
			exitWithError(err)
			return nil
		}

		params := engine.GitConfigParams{
			Scope:      gitconfigScope,
			Regexp:     gitconfigRegexp,
			Section:    gitconfigSection,
			List:       gitconfigList,
			Type:       gitconfigType,
			Structured: gitconfigStructured,
		}

		if len(args) > 0 {
			params.Key = args[0]
		}

		resp, err := eng.GitConfig(gitconfigRepo, params)
		if err != nil {
			exitWithError(err)
			return nil
		}
		writeOutput(resp)
		return nil
	},
}

func init() {
	gitconfigCmd.Flags().StringVar(&gitconfigScope, "scope", "local", "config scope (local, merged, global, system)")
	gitconfigCmd.Flags().StringVar(&gitconfigRegexp, "regexp", "", "match keys by regexp")
	gitconfigCmd.Flags().StringVar(&gitconfigSection, "section", "", "list entries in section (e.g., remote.origin)")
	gitconfigCmd.Flags().BoolVar(&gitconfigList, "list", false, "dump all entries")
	gitconfigCmd.Flags().StringVar(&gitconfigType, "type", "", "type coercion (bool, int, path)")
	gitconfigCmd.Flags().StringVar(&gitconfigStructured, "identity", "", "")
	gitconfigCmd.Flags().StringVar(&gitconfigRepo, "repo", "", "named repo or filesystem path")

	// Convenience flags that set --structured.
	gitconfigCmd.Flags().BoolP("remotes", "", false, "show all remotes (structured)")
	gitconfigCmd.Flags().BoolP("branches", "", false, "show all branches (structured)")

	gitconfigCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("identity") {
			gitconfigStructured = "identity"
		}
		if v, _ := cmd.Flags().GetBool("remotes"); v {
			gitconfigStructured = "remotes"
		}
		if v, _ := cmd.Flags().GetBool("branches"); v {
			gitconfigStructured = "branches"
		}
		return nil
	}

	// Override the --identity flag to be a bool-style flag.
	gitconfigCmd.Flags().Lookup("identity").NoOptDefVal = "identity"

	rootCmd.AddCommand(gitconfigCmd)
}
