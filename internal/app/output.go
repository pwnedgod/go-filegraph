package app

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/pwnedgod/go-selectivetesting"
	"github.com/pwnedgod/go-selectivetesting/internal/util"
)

type testedPackageGroup struct {
	Name       string           `json:"name"`
	TestedPkgs []*testedPackage `json:"testedPkgs"`
}

type testedPackage struct {
	PkgPath         string   `json:"pkgPath"`
	RelativePkgPath string   `json:"relativePkgPath"`
	HasNotable      bool     `json:"hasNotable"`
	TestNames       []string `json:"testNames"`
	RunRegex        string   `json:"runRegex"`
}

func cleanTestedPkgs(basePkg string, crudeTestedPkgs map[string]*selectivetesting.TestedPackage) []*testedPackage {
	testedPkgs := make([]*testedPackage, 0, len(crudeTestedPkgs))
	for pkgPath, tp := range crudeTestedPkgs {
		testNames := tp.Names.ToSlice()
		sort.Strings(testNames)

		runRegex := "^.*"
		if !tp.Names.Has("*") {
			sanitizedTestNames := make([]string, 0, len(testNames))
			for _, testName := range testNames {
				sanitizedTestNames = append(sanitizedTestNames, regexp.QuoteMeta(testName))
			}

			regexPiece := strings.Join(sanitizedTestNames, "|")
			if len(sanitizedTestNames) == 1 {
				runRegex = "^" + regexPiece + "$"
			} else {
				runRegex = "^(" + regexPiece + ")$"
			}
		}

		testedPkgs = append(testedPkgs, &testedPackage{
			PkgPath:         pkgPath,
			RelativePkgPath: util.RelatifyPath(basePkg, pkgPath),
			HasNotable:      tp.HasNotable,
			TestNames:       testNames,
			RunRegex:        runRegex,
		})
	}

	sort.Slice(testedPkgs, func(i, j int) bool {
		return testedPkgs[i].PkgPath < testedPkgs[j].PkgPath
	})

	return testedPkgs
}

func addToGroup(groups map[string]*testedPackageGroup, name string, testedPkg *testedPackage) {
	group := util.MapGetOrCreate(groups, name, func() *testedPackageGroup {
		return &testedPackageGroup{Name: name}
	})
	group.TestedPkgs = append(group.TestedPkgs, testedPkg)
}

func groupBy(combinedTestedPkgs []*testedPackage, pkgPatternGroups []group) []*testedPackageGroup {
	// Marker for package paths that has been grouped.
	grouped := util.NewSet[string]()

	testedPkgGroups := make(map[string]*testedPackageGroup)
	for _, pkgPatternGroup := range pkgPatternGroups {
		for _, testedPkg := range combinedTestedPkgs {
			if grouped.Has(testedPkg.PkgPath) {
				continue
			}
			for _, pattern := range pkgPatternGroup.Patterns {
				if !matchPkgPattern(pattern, testedPkg.PkgPath) {
					continue
				}
				grouped.Add(testedPkg.PkgPath)
				addToGroup(testedPkgGroups, pkgPatternGroup.Name, testedPkg)
			}
		}
	}

	// Handle leftover packages that are not included in any pattern group.
	for _, testedPkg := range combinedTestedPkgs {
		if grouped.Has(testedPkg.PkgPath) {
			continue
		}
		addToGroup(testedPkgGroups, "default", testedPkg)
	}

	cleanedTestedPkgGroups := make([]*testedPackageGroup, 0)
	for _, testedPkgGroup := range testedPkgGroups {
		cleanedTestedPkgGroups = append(cleanedTestedPkgGroups, testedPkgGroup)
	}

	sort.Slice(cleanedTestedPkgGroups, func(i, j int) bool {
		if cleanedTestedPkgGroups[i].Name == "default" {
			return true
		}
		if cleanedTestedPkgGroups[j].Name == "default" {
			return false
		}
		return cleanedTestedPkgGroups[i].Name < cleanedTestedPkgGroups[j].Name
	})

	return cleanedTestedPkgGroups
}

func matchPkgPattern(pkgPattern, pkgPath string) bool {
	if strings.HasSuffix(pkgPattern, "/...") {
		return strings.HasPrefix(pkgPath, pkgPattern[:len(pkgPattern)-4])
	}
	return pkgPattern == pkgPath
}

func jsonTo(out io.Writer, prettyOutput bool, content any) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	if prettyOutput {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(content); err != nil {
		return fmt.Errorf("could not encode json: %w", err)
	}
	return nil
}
