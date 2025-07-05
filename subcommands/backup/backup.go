/*
 * Copyright (c) 2021 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package backup

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/PlakarKorp/kloset/objects"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
	"github.com/PlakarKorp/kloset/snapshot/importer"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands"
	"github.com/PlakarKorp/plakar/utils"
	"github.com/dustin/go-humanize"
	"github.com/gobwas/glob"
)

func init() {
	subcommands.Register(func() subcommands.Subcommand { return &Backup{} }, subcommands.AgentSupport, "backup")
}

type flagList []string

// Called by the flag package to print the default / help.
func (e *flagList) String() string {
	return strings.Join(*e, ",")
}

// Called once per flag occurrence to set the value.
func (e *flagList) Set(value string) error {
	*e = append(*e, value)
	return nil
}

func (cmd *Backup) Parse(ctx *appcontext.AppContext, args []string) error {
	var opt_excludes string
	var opt_exclude flagList
	var opt_tags flagList

	excludes := []string{}

	cmd.Opts = make(map[string]string)

	flags := flag.NewFlagSet("backup", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [OPTIONS] path\n", flags.Name())
		fmt.Fprintf(flags.Output(), "       %s [OPTIONS] @LOCATION\n", flags.Name())
		fmt.Fprintf(flags.Output(), "\nOPTIONS:\n")
		flags.PrintDefaults()
	}

	flags.Uint64Var(&cmd.Concurrency, "concurrency", uint64(ctx.MaxConcurrency), "maximum number of parallel tasks")
	flags.Var(&opt_tags, "tag", "tags to apply to the snapshot, can be specified multiple times to add several tags")
	flags.StringVar(&opt_excludes, "excludes", "", "path to a file containing newline-separated regex patterns, treated as -exclude")
	flags.Var(&opt_exclude, "exclude", "glob pattern to exclude files, can be specified multiple times to add several exclusion patterns")
	flags.BoolVar(&cmd.Quiet, "quiet", false, "suppress output")
	flags.BoolVar(&cmd.Silent, "silent", false, "suppress ALL output")
	flags.BoolVar(&cmd.OptCheck, "check", false, "check the snapshot after creating it")
	flags.Var(utils.NewOptsFlag(cmd.Opts), "o", "specify extra importer options")
	flags.BoolVar(&cmd.DryRun, "scan", false, "do not actually perform a backup, just list the files")
	//flags.BoolVar(&opt_stdio, "stdio", false, "output one line per file to stdout instead of the default interactive output")
	flags.Parse(args)

	for _, item := range opt_exclude {
		if _, err := glob.Compile(item); err != nil {
			return fmt.Errorf("failed to compile exclude pattern: %s", item)
		}
		excludes = append(excludes, item)
	}

	if opt_excludes != "" {
		fp, err := os.Open(opt_excludes)
		if err != nil {
			return fmt.Errorf("unable to open excludes file: %w", err)
		}
		defer fp.Close()

		scanner := bufio.NewScanner(fp)
		for scanner.Scan() {
			line := scanner.Text()
			_, err := glob.Compile(line)
			if err != nil {
				return fmt.Errorf("failed to compile exclude pattern: %s", line)
			}
			excludes = append(excludes, line)
		}
		if err := scanner.Err(); err != nil {
			ctx.GetLogger().Error("%s", err)
			return err
		}
	}

	cmd.RepositorySecret = ctx.GetSecret()
	cmd.Excludes = excludes
	cmd.Path = flags.Arg(0)
	cmd.Tags = opt_tags

	return nil
}

type Backup struct {
	subcommands.SubcommandBase

	Job         string
	Concurrency uint64
	Tags        []string
	Excludes    []string
	Silent      bool
	Quiet       bool
	Path        string
	OptCheck    bool
	Opts        map[string]string
	DryRun      bool
}

func (cmd *Backup) Execute(ctx *appcontext.AppContext, repo *repository.Repository) (int, error) {
	ret, err, _, _ := cmd.DoBackup(ctx, repo)
	return ret, err
}

func (cmd *Backup) DoBackup(ctx *appcontext.AppContext, repo *repository.Repository) (int, error, objects.MAC, error) {

	excludes := []glob.Glob{}
	for _, item := range cmd.Excludes {
		g, err := glob.Compile(item)
		if err != nil {
			return 1, fmt.Errorf("failed to compile exclude pattern: %s", item), objects.MAC{}, nil
		}
		excludes = append(excludes, g)
	}

	opts := &snapshot.BackupOptions{
		MaxConcurrency: cmd.Concurrency,
		Name:           "default",
		Tags:           cmd.Tags,
		Excludes:       excludes,
	}

	scanDir := "fs:" + ctx.CWD
	if cmd.Path != "" {
		scanDir = cmd.Path
	}

	if strings.HasPrefix(scanDir, "@") {
		remote, ok := ctx.Config.GetSource(scanDir[1:])
		if !ok {
			return 1, fmt.Errorf("could not resolve importer: %s", scanDir), objects.MAC{}, nil
		}
		if _, ok := remote["location"]; !ok {
			return 1, fmt.Errorf("could not resolve importer location: %s", scanDir), objects.MAC{}, nil
		} else {
			// inherit all the options -- but the ones
			// specified in the command line takes the
			// precendence.
			for k, v := range remote {
				if _, found := cmd.Opts[k]; !found {
					cmd.Opts[k] = v
				}
			}
		}
	}

	// Now that we have resolved the possible @ syntax let's apply the scandir.
	if _, found := cmd.Opts["location"]; !found {
		cmd.Opts["location"] = scanDir
	}

	imp, err := importer.NewImporter(ctx.GetInner(), ctx.ImporterOpts(), cmd.Opts)
	if err != nil {
		return 1, fmt.Errorf("failed to create an importer for %s: %s", scanDir, err), objects.MAC{}, nil
	}
	defer imp.Close()

	if cmd.DryRun {
		if err := dryrun(ctx, imp, excludes); err != nil {
			return 1, err, objects.MAC{}, nil
		}
		return 0, nil, objects.MAC{}, nil
	}

	snap, err := snapshot.Create(repo, repository.DefaultType)
	if err != nil {
		ctx.GetLogger().Error("%s", err)
		return 1, err, objects.MAC{}, nil
	}
	defer snap.Close()

	if cmd.Job != "" {
		snap.Header.Job = cmd.Job
	}

	if cmd.Silent {
		if err := snap.Backup(imp, opts); err != nil {
			return 1, fmt.Errorf("failed to create snapshot: %w", err), objects.MAC{}, nil
		}
	} else {
		ep := startEventsProcessor(ctx, imp.Root(), true, cmd.Quiet)
		if err := snap.Backup(imp, opts); err != nil {
			ep.Close()
			return 1, fmt.Errorf("failed to create snapshot: %w", err), objects.MAC{}, nil
		}
		ep.Close()
	}

	if cmd.OptCheck {
		repo.RebuildState()

		checkOptions := &snapshot.CheckOptions{
			MaxConcurrency: cmd.Concurrency,
			FastCheck:      false,
		}

		checkSnap, err := snapshot.Load(repo, snap.Header.Identifier)
		if err != nil {
			return 1, fmt.Errorf("failed to load snapshot: %w", err), objects.MAC{}, nil
		}
		defer checkSnap.Close()

		checkCache, err := ctx.GetCache().Check()
		if err != nil {
			return 1, err, objects.MAC{}, nil
		}
		defer checkCache.Close()

		checkSnap.SetCheckCache(checkCache)

		if err := checkSnap.Check("/", checkOptions); err != nil {
			return 1, fmt.Errorf("failed to check snapshot: %w", err), objects.MAC{}, nil
		}
	}

	totalSize := snap.Header.GetSource(0).Summary.Directory.Size + snap.Header.GetSource(0).Summary.Below.Size

	ctx.GetLogger().Info("backup: created %s snapshot %x of size %s in %s (wrote %s)",
		"unsigned",
		snap.Header.GetIndexShortID(),
		humanize.Bytes(totalSize),
		snap.Header.Duration,
		humanize.Bytes(uint64(snap.Repository().WBytes())),
	)

	totalErrors := uint64(0)
	for i := 0; i < len(snap.Header.Sources); i++ {
		s := snap.Header.GetSource(i)
		totalErrors += s.Summary.Directory.Errors + s.Summary.Below.Errors
	}
	var warning error
	if totalErrors > 0 {
		warning = fmt.Errorf("%d errors during backup", totalErrors)
	}
	return 0, nil, snap.Header.Identifier, warning
}

func dryrun(ctx *appcontext.AppContext, imp importer.Importer, excludes []glob.Glob) error {
	scanner, err := imp.Scan()
	if err != nil {
		return fmt.Errorf("failed to scan: %w", err)
	}

	errors := false
	for record := range scanner {
		var pathname string
		switch {
		case record.Record != nil:
			pathname = record.Record.Pathname
		case record.Error != nil:
			pathname = record.Error.Pathname
		}

		skip := false
		for _, exclude := range excludes {
			if exclude.Match(pathname) {
				skip = true
				break
			}
		}
		if skip {
			if record.Record != nil {
				record.Record.Close()
			}
			continue
		}

		switch {
		case record.Error != nil:
			errors = true
			fmt.Fprintf(ctx.Stderr, "%s: %s\n",
				record.Error.Pathname, record.Error.Err)
		case record.Record != nil:
			fmt.Fprintln(ctx.Stdout, record.Record.Pathname)
			record.Record.Close()
		}
	}

	if errors {
		return fmt.Errorf("failed to scan some files")
	}
	return nil
}
