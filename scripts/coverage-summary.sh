#!/usr/bin/env bash
# A Go coverage profile summed by package, as a Markdown table. CI adds it to
# the integration job's summary: the integration tests are the tests of
# internal/service and internal/web, so unit coverage says little of them.
#
#   make test-integration-cover          # writes coverage.out and runs this
#   scripts/coverage-summary.sh coverage.out
set -euo pipefail

profile=${1:-coverage.out}
module=$(go list -m)

echo "| Package | Statements | Covered |"
echo "|:--|--:|--:|"
awk -F'[: ]' -v module="$module/" '
	NR == 1 { next } # the mode line
	{
		pkg = $1
		sub(/\/[^\/]*$/, "", pkg)
		if (index(pkg, module) == 1) pkg = substr(pkg, length(module) + 1)
		total[pkg] += $3; all += $3
		if ($4 > 0) { covered[pkg] += $3; hit += $3 }
	}
	END {
		if (all == 0) { print "no statements in " FILENAME > "/dev/stderr"; exit 1 }
		for (p in total) printf "| %s | %d | %.1f%% |\n", p, total[p], 100 * covered[p] / total[p] | "sort"
		close("sort")
		printf "| All | %d | %.1f%% |\n", all, 100 * hit / all
	}' "$profile"
