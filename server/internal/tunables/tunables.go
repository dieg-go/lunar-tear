// Package tunables applies operator overrides to values that would otherwise be
// compiled in: the gacha rarity weights, its prices and pity, and the sizing of
// the chapter (box) gacha.
//
// The file is optional. Without one, every value keeps the default the binary
// was built with, so a stock install behaves exactly as before; with one, an
// operator can retune a private server without rebuilding it.
//
// Fields are matched strictly: an unknown key or an out-of-range value aborts
// startup with the offending field named. A tuning file that silently does
// nothing is worse than one that refuses to run.
package tunables

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"lunar-tear/server/internal/gacha"
	"lunar-tear/server/internal/model"
)

// Tunables is the shape of the JSON file. Every field may be omitted; omitted
// fields keep their built-in default.
type Tunables struct {
	Gacha Gacha `json:"gacha"`
}

// Gacha covers the premium, step-up and box draws.
type Gacha struct {
	// RarityWeights are relative weights, not percentages: a draw picks against
	// their sum. The built-in 200/300/500/1000/8000 out of 10000 is 2% costume
	// SSR, 3% weapon SSR, 5% costume SR, 10% weapon SR and 80% weapon R.
	CostumeSSRWeight int `json:"costumeSSRWeight"`
	WeaponSSRWeight  int `json:"weaponSSRWeight"`
	CostumeSRWeight  int `json:"costumeSRWeight"`
	WeaponSRWeight   int `json:"weaponSRWeight"`
	WeaponRWeight    int `json:"weaponRWeight"`

	// FeaturedRateUpPercent is the chance, in percent, that a draw landing on a
	// featured unit's rarity tier yields that unit rather than a random one.
	FeaturedRateUpPercent int `json:"featuredRateUpPercent"`

	// PityCeilingCount is the medal count at which a banner pays out regardless.
	PityCeilingCount int32 `json:"pityCeilingCount"`

	// Premium prices are in gems and PremiumMultiPullCount is how many items a
	// multi pull draws. Zero makes a pull free.
	PremiumSinglePullPrice int32 `json:"premiumSinglePullPrice"`
	PremiumMultiPullPrice  int32 `json:"premiumMultiPullPrice"`
	PremiumMultiPullCount  int32 `json:"premiumMultiPullCount"`

	// Step-up banner costs, in paid gems. The second and fourth steps are free
	// and are not tunable.
	StepUpStep1Cost int32 `json:"stepUpStep1Cost"`
	StepUpStep3Cost int32 `json:"stepUpStep3Cost"`
	StepUpStep5Cost int32 `json:"stepUpStep5Cost"`

	// StepUpRateBoost multiplies the SSR weights on steps 1 and 3;
	// StepUpRateMaxBoost does the same on step 5.
	StepUpRateBoost    float64 `json:"stepUpRateBoost"`
	StepUpRateMaxBoost float64 `json:"stepUpRateMaxBoost"`

	// Duplicate conversions roll grade = DuplicateGradeMin +
	// rand(DuplicateGradeRange).
	DuplicateGradeMin   int32 `json:"duplicateGradeMin"`
	DuplicateGradeRange int   `json:"duplicateGradeRange"`

	// DailyDrawLimit caps the reward draw when master data sets no limit of its
	// own.
	DailyDrawLimit int32 `json:"dailyDrawLimit"`

	// Box sizing for the chapter gacha: each material is placed
	// BoxItemDefaultMax times, so a box holds at most BoxPoolMaxItems entries and
	// is padded to BoxPoolMinItems.
	BoxPoolMaxItems    int   `json:"boxPoolMaxItems"`
	BoxPoolMinItems    int   `json:"boxPoolMinItems"`
	BoxItemDefaultMax  int32 `json:"boxItemDefaultMax"`
	BoxFallbackItemMax int32 `json:"boxFallbackItemMax"`
}

// Default returns the built-in values, read from the packages that own them so
// that changing a default stays a one-place edit.
func Default() Tunables {
	return Tunables{Gacha: Gacha{
		CostumeSSRWeight:       rateWeight(int32(model.PossessionTypeCostume), model.RaritySSRare),
		WeaponSSRWeight:        rateWeight(int32(model.PossessionTypeWeapon), model.RaritySSRare),
		CostumeSRWeight:        rateWeight(int32(model.PossessionTypeCostume), model.RaritySRare),
		WeaponSRWeight:         rateWeight(int32(model.PossessionTypeWeapon), model.RaritySRare),
		WeaponRWeight:          rateWeight(int32(model.PossessionTypeWeapon), model.RarityRare),
		FeaturedRateUpPercent:  model.FeaturedRateUpPercent,
		PityCeilingCount:       model.PityCeilingCount,
		PremiumSinglePullPrice: model.PremiumSinglePullPrice,
		PremiumMultiPullPrice:  model.PremiumMultiPullPrice,
		PremiumMultiPullCount:  model.PremiumMultiPullCount,
		StepUpStep1Cost:        model.StepUpStep1Cost,
		StepUpStep3Cost:        model.StepUpStep3Cost,
		StepUpStep5Cost:        model.StepUpStep5Cost,
		StepUpRateBoost:        model.StepUpRateBoost,
		StepUpRateMaxBoost:     model.StepUpRateMaxBoost,
		DuplicateGradeMin:      model.DupGradeMin,
		DuplicateGradeRange:    model.DupGradeRange,
		DailyDrawLimit:         model.DefaultDailyDrawLimit,
		BoxPoolMaxItems:        model.BoxPoolMaxItems,
		BoxPoolMinItems:        model.BoxPoolMinItems,
		BoxItemDefaultMax:      model.BoxItemDefaultMax,
		BoxFallbackItemMax:     model.BoxFallbackItemMax,
	}}
}

// Load reads path over the defaults. A missing, empty or whitespace-only file is
// not an error: it means "use the built-in values", which is what every install
// has until an operator writes one.
func Load(path string) (Tunables, error) {
	t := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return t, nil
	}
	if err != nil {
		return t, fmt.Errorf("read: %w", err)
	}
	// Editors on Windows (Notepad, PowerShell's Set-Content -Encoding utf8) write
	// a byte order mark. Refusing to start over an invisible leading byte would be
	// hostile, so it is dropped here.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(data)) == 0 {
		return t, nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return t, fmt.Errorf("parse: %w", err)
	}
	// A second value means trailing content, which Decode alone would ignore.
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return t, errors.New("parse: unexpected content after the JSON object")
	}
	if err := t.validate(); err != nil {
		return t, err
	}
	return t, nil
}

func (t Tunables) validate() error {
	g := t.Gacha

	weights := []struct {
		name   string
		weight int
	}{
		{"costumeSSRWeight", g.CostumeSSRWeight},
		{"weaponSSRWeight", g.WeaponSSRWeight},
		{"costumeSRWeight", g.CostumeSRWeight},
		{"weaponSRWeight", g.WeaponSRWeight},
		{"weaponRWeight", g.WeaponRWeight},
	}
	total := 0
	for _, w := range weights {
		if w.weight < 0 {
			return fmt.Errorf("%s must not be negative", w.name)
		}
		total += w.weight
	}
	if total == 0 {
		return errors.New("rarity weights must not all be zero: a draw divides by their sum")
	}

	if g.FeaturedRateUpPercent < 0 || g.FeaturedRateUpPercent > 100 {
		return errors.New("featuredRateUpPercent must be between 0 and 100")
	}
	if g.PityCeilingCount < 1 {
		return errors.New("pityCeilingCount must be at least 1")
	}
	if g.PremiumSinglePullPrice < 0 || g.PremiumMultiPullPrice < 0 {
		return errors.New("premium pull prices must not be negative")
	}
	if g.PremiumMultiPullCount < 1 {
		return errors.New("premiumMultiPullCount must be at least 1")
	}
	if g.StepUpStep1Cost < 0 || g.StepUpStep3Cost < 0 || g.StepUpStep5Cost < 0 {
		return errors.New("step-up costs must not be negative")
	}
	if g.StepUpRateBoost < 0 || g.StepUpRateBoost > 100 || g.StepUpRateMaxBoost < 0 || g.StepUpRateMaxBoost > 100 {
		return errors.New("step-up rate boosts must be between 0 and 100")
	}
	if g.DuplicateGradeMin < 0 {
		return errors.New("duplicateGradeMin must not be negative")
	}
	if g.DuplicateGradeRange < 1 {
		return errors.New("duplicateGradeRange must be at least 1")
	}
	if g.DailyDrawLimit < 1 {
		return errors.New("dailyDrawLimit must be at least 1")
	}
	if g.BoxPoolMaxItems < 1 || g.BoxPoolMinItems < 1 {
		return errors.New("box pool sizes must be at least 1")
	}
	if g.BoxPoolMinItems > g.BoxPoolMaxItems {
		return errors.New("boxPoolMinItems must not exceed boxPoolMaxItems")
	}
	if g.BoxItemDefaultMax < 1 || g.BoxFallbackItemMax < 1 {
		return errors.New("box item counts must be at least 1")
	}
	return nil
}

// Apply writes the values into the packages that own them. It must run before
// the runtime holder is built: gacha prices and pity are baked into the banner
// catalog when master data is loaded.
func (t Tunables) Apply() {
	g := t.Gacha

	setRateWeight(int32(model.PossessionTypeCostume), model.RaritySSRare, g.CostumeSSRWeight)
	setRateWeight(int32(model.PossessionTypeWeapon), model.RaritySSRare, g.WeaponSSRWeight)
	setRateWeight(int32(model.PossessionTypeCostume), model.RaritySRare, g.CostumeSRWeight)
	setRateWeight(int32(model.PossessionTypeWeapon), model.RaritySRare, g.WeaponSRWeight)
	setRateWeight(int32(model.PossessionTypeWeapon), model.RarityRare, g.WeaponRWeight)

	model.FeaturedRateUpPercent = g.FeaturedRateUpPercent
	model.PityCeilingCount = g.PityCeilingCount
	model.PremiumSinglePullPrice = g.PremiumSinglePullPrice
	model.PremiumMultiPullPrice = g.PremiumMultiPullPrice
	model.PremiumMultiPullCount = g.PremiumMultiPullCount
	model.StepUpStep1Cost = g.StepUpStep1Cost
	model.StepUpStep3Cost = g.StepUpStep3Cost
	model.StepUpStep5Cost = g.StepUpStep5Cost
	model.StepUpRateBoost = g.StepUpRateBoost
	model.StepUpRateMaxBoost = g.StepUpRateMaxBoost
	model.DupGradeMin = g.DuplicateGradeMin
	model.DupGradeRange = g.DuplicateGradeRange
	model.DefaultDailyDrawLimit = g.DailyDrawLimit
	model.BoxPoolMaxItems = g.BoxPoolMaxItems
	model.BoxPoolMinItems = g.BoxPoolMinItems
	model.BoxItemDefaultMax = g.BoxItemDefaultMax
	model.BoxFallbackItemMax = g.BoxFallbackItemMax
}

// Summary describes the effective odds and prices on one line, for the startup
// log: a phone has no shell to check a config file with.
func (t Tunables) Summary() string {
	g := t.Gacha
	total := g.CostumeSSRWeight + g.WeaponSSRWeight + g.CostumeSRWeight + g.WeaponSRWeight + g.WeaponRWeight
	share := func(weight int) float64 {
		if total == 0 {
			return 0
		}
		return float64(weight) * 100 / float64(total)
	}
	return fmt.Sprintf(
		"SSR %.2f%% (costume %.2f%%, weapon %.2f%%), SR %.2f%%, featured rate-up %d%%, pity %d, single %d gems, multi %d gems for %d, step-up boost %.2fx/%.2fx",
		share(g.CostumeSSRWeight+g.WeaponSSRWeight), share(g.CostumeSSRWeight), share(g.WeaponSSRWeight),
		share(g.CostumeSRWeight+g.WeaponSRWeight), g.FeaturedRateUpPercent, g.PityCeilingCount,
		g.PremiumSinglePullPrice, g.PremiumMultiPullPrice, g.PremiumMultiPullCount,
		g.StepUpRateBoost, g.StepUpRateMaxBoost,
	)
}

// rateWeight looks a tier up by possession type and rarity rather than by index,
// so the JSON field names cannot fall out of step with the order of
// gacha.PremiumRates.
func rateWeight(possessionType int32, rarity model.RarityType) int {
	for _, tier := range gacha.PremiumRates {
		if tier.PossessionType == possessionType && tier.RarityType == rarity {
			return tier.Weight
		}
	}
	return 0
}

func setRateWeight(possessionType int32, rarity model.RarityType, weight int) {
	for i := range gacha.PremiumRates {
		if gacha.PremiumRates[i].PossessionType == possessionType && gacha.PremiumRates[i].RarityType == rarity {
			gacha.PremiumRates[i].Weight = weight
			return
		}
	}
}
