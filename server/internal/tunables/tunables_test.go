package tunables

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lunar-tear/server/internal/gacha"
	"lunar-tear/server/internal/masterdata"
	"lunar-tear/server/internal/model"
)

// guard restores every process-wide value Apply writes to, so one test's
// overrides cannot leak into the next.
func guard(t *testing.T) {
	t.Helper()
	rates := append([]gacha.RateTier(nil), gacha.PremiumRates...)
	before := Default()
	t.Cleanup(func() {
		copy(gacha.PremiumRates, rates)
		before.Apply()
	})
}

func writeFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tunables.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFallsBackToDefaults(t *testing.T) {
	guard(t)

	for name, path := range map[string]string{
		"missing":         filepath.Join(t.TempDir(), "absent.json"),
		"empty":           writeFile(t, ""),
		"whitespace":      writeFile(t, "\n\t \n"),
		"empty object":    writeFile(t, "{}"),
		"missing nested":  writeFile(t, `{"gacha":{}}`),
		"byte order mark": writeFile(t, "\ufeff"),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Load(path)
			if err != nil {
				t.Fatalf("Load(%s) = %v, want defaults", path, err)
			}
			if got != Default() {
				t.Errorf("Load(%s) = %+v, want the built-in defaults %+v", path, got, Default())
			}
		})
	}
}

func TestLoadKeepsDefaultsForOmittedFields(t *testing.T) {
	guard(t)

	got, err := Load(writeFile(t, `{"gacha":{"pityCeilingCount":10,"weaponRWeight":1}}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Gacha.PityCeilingCount != 10 || got.Gacha.WeaponRWeight != 1 {
		t.Errorf("overrides not applied: %+v", got.Gacha)
	}
	want := Default().Gacha
	want.PityCeilingCount, want.WeaponRWeight = 10, 1
	if got.Gacha != want {
		t.Errorf("omitted fields changed: got %+v, want %+v", got.Gacha, want)
	}
}

func TestLoadToleratesUTF8ByteOrderMark(t *testing.T) {
	guard(t)

	// Notepad and PowerShell's Set-Content -Encoding utf8 both write one, and the
	// rest of the file is perfectly good.
	got, err := Load(writeFile(t, "\ufeff{\"gacha\":{\"pityCeilingCount\":10}}"))
	if err != nil {
		t.Fatalf("Load with a byte order mark: %v", err)
	}
	if got.Gacha.PityCeilingCount != 10 {
		t.Errorf("pityCeilingCount = %d, want 10", got.Gacha.PityCeilingCount)
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	guard(t)

	cases := map[string]string{
		"unknown top-level field": `{"gachaRate":200}`,
		"unknown gacha field":     `{"gacha":{"pityCeiling":10}}`,
		"all weights zero":        `{"gacha":{"costumeSSRWeight":0,"weaponSSRWeight":0,"costumeSRWeight":0,"weaponSRWeight":0,"weaponRWeight":0}}`,
		"negative weight":         `{"gacha":{"weaponRWeight":-1}}`,
		"percent over 100":        `{"gacha":{"featuredRateUpPercent":101}}`,
		"pity zero":               `{"gacha":{"pityCeilingCount":0}}`,
		"negative price":          `{"gacha":{"premiumMultiPullPrice":-1}}`,
		"multi pull count zero":   `{"gacha":{"premiumMultiPullCount":0}}`,
		"zero grade range":        `{"gacha":{"duplicateGradeRange":0}}`,
		"box min over max":        `{"gacha":{"boxPoolMinItems":99,"boxPoolMaxItems":10}}`,
		"trailing object":         `{"gacha":{}}{"gacha":{}}`,
		"not json":                `gacha: 200`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeFile(t, body)); err == nil {
				t.Fatalf("Load(%s) succeeded, want an error", body)
			}
		})
	}
}

func TestApplyReachesTheDraw(t *testing.T) {
	guard(t)

	// Every draw is a weapon SSR, so the knob has to show up in the result and
	// not merely in a variable.
	tun, err := Load(writeFile(t, `{"gacha":{"costumeSSRWeight":0,"weaponSSRWeight":100,"costumeSRWeight":0,"weaponSRWeight":0,"weaponRWeight":0}}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tun.Apply()
	if v := rateOf(t, int32(model.PossessionTypeWeapon), model.RaritySSRare); v != 100 {
		t.Fatalf("weapon SSR weight = %d, want 100 after Apply", v)
	}

	pool := &masterdata.BannerPool{
		WeaponsByRarity: map[int32][]masterdata.GachaPoolItem{
			int32(model.RaritySSRare): {{PossessionType: int32(model.PossessionTypeWeapon), PossessionId: 42, RarityType: model.RaritySSRare}},
		},
	}
	for i, item := range gacha.DrawPremium(pool, 20, 0, 0, 1.0) {
		if item.PossessionId != 42 || item.RarityType != model.RaritySSRare {
			t.Fatalf("draw %d = %+v, want the only SSR in the pool", i, item)
		}
	}
}

func TestApplyWritesPricesAndPity(t *testing.T) {
	guard(t)

	tun, err := Load(writeFile(t, `{"gacha":{"premiumSinglePullPrice":7,"premiumMultiPullPrice":70,"premiumMultiPullCount":7,"pityCeilingCount":11,"stepUpRateMaxBoost":9.5,"boxPoolMinItems":2,"boxPoolMaxItems":3,"dailyDrawLimit":2}}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tun.Apply()

	if model.PremiumSinglePullPrice != 7 || model.PremiumMultiPullPrice != 70 || model.PremiumMultiPullCount != 7 {
		t.Errorf("prices = %d/%d for %d, want 7/70 for 7", model.PremiumSinglePullPrice, model.PremiumMultiPullPrice, model.PremiumMultiPullCount)
	}
	if model.PityCeilingCount != 11 || model.StepUpRateMaxBoost != 9.5 || model.BoxPoolMaxItems != 3 || model.DefaultDailyDrawLimit != 2 {
		t.Errorf("pity/boost/box/limit = %d/%v/%d/%d, want 11/9.5/3/2", model.PityCeilingCount, model.StepUpRateMaxBoost, model.BoxPoolMaxItems, model.DefaultDailyDrawLimit)
	}
}

func TestSummaryNamesTheOdds(t *testing.T) {
	guard(t)

	summary := Default().Summary()
	for _, want := range []string{"SSR 5.00%", "costume 2.00%", "weapon 3.00%", "SR 15.00%"} {
		if !strings.Contains(summary, want) {
			t.Errorf("Summary() = %q, want it to contain %q", summary, want)
		}
	}
}

func rateOf(t *testing.T, possessionType int32, rarity model.RarityType) int {
	t.Helper()
	for _, tier := range gacha.PremiumRates {
		if tier.PossessionType == possessionType && tier.RarityType == rarity {
			return tier.Weight
		}
	}
	t.Fatalf("no rate tier for possession type %d, rarity %d", possessionType, rarity)
	return 0
}
