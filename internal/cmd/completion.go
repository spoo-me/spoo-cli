package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/spoo-me/spoo-cli/internal/api"
)

// completionTimeout bounds the network fetch behind a Tab press so the
// shell never hangs waiting on the API.
const completionTimeout = 2 * time.Second

// completionContext bounds a completion fetch so the shell never hangs on
// a Tab press, falling back to a fresh background context when cobra hasn't
// attached one.
func completionContext(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, completionTimeout)
}

// completionURLs fetches the signed-in user's links for shell completion.
// It is best-effort: any failure (not logged in, offline, timeout) yields
// no items, so completion silently offers nothing instead of erroring.
func completionURLs(cmd *cobra.Command) []api.URLItem {
	d, err := newDeps()
	if err != nil {
		return nil
	}
	ctx, cancel := completionContext(cmd)
	defer cancel()
	page, err := d.client.ListURLs(ctx, api.ListURLsOptions{
		PageSize: 100, SortBy: "last_click", SortOrder: "descending",
	})
	if err != nil {
		return nil
	}
	return page.Items
}

// completeAlias completes a single short-code argument with the user's
// aliases, each annotated with its destination. Used by the commands that
// act on a link by its alias (open, inspect, qr, stats, export).
func completeAlias(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 { // these commands take at most one code
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, it := range completionURLs(cmd) {
		if it.Alias != "" && strings.HasPrefix(it.Alias, toComplete) {
			out = append(out, it.Alias+"\t"+truncate(it.LongURL, 48))
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeLinkID completes the opaque <id> argument of `links update` and
// `links delete`. IDs aren't memorable, so each is described by its alias
// to make the right one pickable.
func completeLinkID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, it := range completionURLs(cmd) {
		if strings.HasPrefix(it.ID, toComplete) {
			out = append(out, it.ID+"\t"+it.Alias)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completeKeyID completes the <id> of `keys revoke`, described by the key
// name and skipping already-revoked keys.
func completeKeyID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	d, err := newDeps()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, cancel := completionContext(cmd)
	defer cancel()
	keys, err := d.client.ListKeys(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, k := range keys {
		if k.Revoked || !strings.HasPrefix(k.ID, toComplete) {
			continue
		}
		out = append(out, k.ID+"\t"+k.Name)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// apiScopes are the permission scopes accepted by `keys create --scopes`,
// mirroring the set documented in that command's help.
var apiScopes = []string{
	"shorten:create", "urls:read", "urls:manage", "stats:read",
	"domains:read", "domains:manage", "admin:all",
}

// completeScopes completes the comma-separated --scopes value: it keeps the
// scopes already typed and offers the rest. cobra hands the whole value as
// toComplete for slice flags, so we split on the last comma ourselves and
// use NoSpace so the user can keep appending.
func completeScopes(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	prefix, cur := "", toComplete
	if i := strings.LastIndex(toComplete, ","); i >= 0 {
		prefix, cur = toComplete[:i+1], toComplete[i+1:]
	}
	chosen := make(map[string]bool)
	for _, s := range strings.Split(prefix, ",") {
		chosen[s] = true
	}
	var out []string
	for _, s := range apiScopes {
		if !chosen[s] && strings.HasPrefix(s, cur) {
			out = append(out, prefix+s)
		}
	}
	return out, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// completeDomain completes --domain with the distinct custom domains that
// already appear on your links. There's no domains-list endpoint, so this
// is the practical best-effort source (a domain you've never used yet
// won't show, but you can still type it).
func completeDomain(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	seen := make(map[string]bool)
	var out []string
	for _, it := range completionURLs(cmd) {
		if it.Domain != "" && !seen[it.Domain] && strings.HasPrefix(it.Domain, toComplete) {
			seen[it.Domain] = true
			out = append(out, it.Domain)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// flagComp registers a completion function for a flag, ignoring the error
// cobra returns only for an unknown flag name (a bug the tests catch).
func flagComp(cmd *cobra.Command, flag string, fn func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)) {
	_ = cmd.RegisterFlagCompletionFunc(flag, fn)
}

// fixed registers a flag's fixed set of allowed values for completion.
func fixed(cmd *cobra.Command, flag string, values ...string) {
	flagComp(cmd, flag, cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp))
}
