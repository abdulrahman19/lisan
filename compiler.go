package lisan

import (
	"cmp"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"golang.org/x/text/language"
)

// translationExt is the extension every translation file must carry.
const translationExt = ".json"

// compiler turns a translation tree into a catalog, collecting every defect it
// finds along the way rather than stopping at the first.
type compiler struct {
	fsys     fs.FS
	regions  map[string]*regionData
	config   configFile
	problems []Problem
}

// compile validates an entire translation tree and returns it ready for use.
// It reports a *CompileError listing every problem found.
func compile(fsys fs.FS) (*catalog, error) {
	worker := &compiler{fsys: fsys, regions: make(map[string]*regionData)}

	return worker.run()
}

// run executes the compilation phases in order, stopping early only when the
// configuration itself is unusable.
func (c *compiler) run() (*catalog, error) {
	config, problems := loadConfig(c.fsys)

	c.config = config
	c.problems = append(c.problems, problems...)

	if len(c.problems) > 0 {
		return nil, c.fail()
	}

	c.loadRegions()
	c.checkUndeclaredFolders()
	c.validateAgainstBase()

	if len(c.problems) > 0 {
		return nil, c.fail()
	}

	return c.buildCatalog(), nil
}

// loadRegions compiles every region declared in the configuration.
func (c *compiler) loadRegions() {
	for _, code := range sortedKeys(c.config.Regions) {
		if region := c.loadRegion(code, c.config.Regions[code]); region != nil {
			c.regions[code] = region
		}
	}
}

// loadRegion resolves a region's language, its required plural categories, and
// every translation file in its folder.
func (c *compiler) loadRegion(code string, declared regionConfig) *regionData {
	tag, err := language.Parse(code)
	if err != nil {
		c.problemf(configName, textPosition{}, "", "region %q is not a valid BCP 47 tag: %v", code, err)

		return nil
	}

	region := &regionData{
		entries:  make(map[string]*entry),
		rejected: make(map[string]*entry),
		code:     code,
		name:     declared.Name,
		locales:  slices.Clone(declared.Locales),
		forms:    categoriesFor(tag),
		tag:      tag,
	}

	c.checkLocaleTags(region)
	c.loadRegionFiles(region)

	return region
}

// checkLocaleTags rejects a region whose locales do not all share its plural
// categories, since they would otherwise resolve to a form the files never
// declare.
func (c *compiler) checkLocaleTags(region *regionData) {
	want := formNames(region.forms)

	for _, locale := range region.locales {
		tag, err := language.Parse(locale)
		if err != nil {
			c.problemf(configName, textPosition{}, "",
				"region %q lists locale %q, which is not a valid BCP 47 tag: %v", region.code, locale, err)

			continue
		}

		if got := formNames(categoriesFor(tag)); !slices.Equal(got, want) {
			c.problemf(configName, textPosition{}, "",
				"region %q requires categories [%s] but its locale %q requires [%s], so they cannot share one translation set",
				region.code, strings.Join(want, " "), locale, strings.Join(got, " "))
		}
	}
}

// loadRegionFiles reads every translation file under a region's folder.
func (c *compiler) loadRegionFiles(region *regionData) {
	info, err := fs.Stat(c.fsys, region.code)
	if err != nil || !info.IsDir() {
		c.problemf(region.code, textPosition{}, "", "region %q has no folder in the translation tree", region.code)

		return
	}

	if walkErr := fs.WalkDir(c.fsys, region.code, c.visitFile(region)); walkErr != nil {
		c.problemf(region.code, textPosition{}, "", "could not be read: %v", walkErr)
	}
}

// visitFile returns a walk function that loads every translation file it meets.
func (c *compiler) visitFile(region *regionData) fs.WalkDirFunc {
	return func(name string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !item.IsDir() && strings.HasSuffix(item.Name(), translationExt) {
			c.loadFile(region, name)
		}

		return nil
	}
}

// loadFile decodes one translation file and adds its entries to the region.
func (c *compiler) loadFile(region *regionData, name string) {
	data, err := fs.ReadFile(c.fsys, name)
	if err != nil {
		c.problemf(name, textPosition{}, "", "could not be read: %v", err)

		return
	}

	file := newSourceFile(name, data)

	raws, offset, err := readEntries(data)
	if err != nil {
		c.problemf(name, file.position(offset), "", "%v", err)

		return
	}

	for _, raw := range raws {
		c.addEntry(region, file, raw)
	}
}

// checkUndeclaredFolders rejects a folder in the tree that no region claims,
// which is almost always a typo in a region code.
func (c *compiler) checkUndeclaredFolders() {
	items, err := fs.ReadDir(c.fsys, ".")
	if err != nil {
		c.problemf(".", textPosition{}, "", "could not be read: %v", err)

		return
	}

	for _, item := range items {
		if !item.IsDir() {
			continue
		}

		if _, declared := c.config.Regions[item.Name()]; !declared {
			c.problemf(item.Name(), textPosition{}, "",
				"folder %q is not declared under regions in %s", item.Name(), configName)
		}
	}
}

// buildCatalog assembles the compiled regions into a ready-to-use catalog.
func (c *compiler) buildCatalog() *catalog {
	built := &catalog{
		regions:  c.regions,
		byLocale: make(map[string]localeBinding, len(c.regions)),
		order:    sortedKeys(c.regions),
		baseCode: c.config.Settings.BaseRegion,
	}

	for _, code := range built.order {
		region := c.regions[code]
		built.byLocale[code] = localeBinding{region: region, code: code, tag: region.tag}

		for _, locale := range region.locales {
			tag, err := language.Parse(locale)
			if err != nil {
				continue
			}

			built.byLocale[locale] = localeBinding{region: region, code: locale, tag: tag}
		}
	}

	built.matchCodes = matchOrder(built)

	tags := make([]language.Tag, 0, len(built.matchCodes))
	for _, code := range built.matchCodes {
		tags = append(tags, built.byLocale[code].tag)
	}

	built.matcher = language.NewMatcher(tags)

	return built
}

// matchOrder lists every bindable code with the base region first, because
// language.Matcher treats its first tag as the fallback.
func matchOrder(built *catalog) []string {
	codes := make([]string, 0, len(built.byLocale))
	codes = append(codes, built.baseCode)

	for _, code := range sortedKeys(built.byLocale) {
		if code != built.baseCode {
			codes = append(codes, code)
		}
	}

	return codes
}

// problemf records one defect.
func (c *compiler) problemf(file string, pos textPosition, id, format string, args ...any) {
	c.problems = append(c.problems, Problem{
		File:    file,
		ID:      id,
		Message: fmt.Sprintf(format, args...),
		Line:    pos.line,
		Column:  pos.column,
	})
}

// fail returns the accumulated problems in a stable, file-ordered error.
func (c *compiler) fail() error {
	slices.SortStableFunc(c.problems, func(a, b Problem) int {
		return cmp.Or(
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Column, b.Column),
			cmp.Compare(a.ID, b.ID),
		)
	})

	return &CompileError{Problems: c.problems}
}
