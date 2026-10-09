package model

const (
	GachaLabelUnknown    int32 = 0
	GachaLabelPremium    int32 = 1
	GachaLabelEvent      int32 = 2
	GachaLabelChapter    int32 = 3
	GachaLabelPortalCage int32 = 4
	GachaLabelRecycle    int32 = 5
)

const (
	GachaModeUnknown int32 = 0
	GachaModeBasic   int32 = 1
	GachaModeStepup  int32 = 2
	GachaModeBox     int32 = 3
)

const (
	GachaUnlockUnknown                  int32 = 0
	GachaUnlockNone                     int32 = 1
	GachaUnlockUserRankGreaterOrEqual   int32 = 2
	GachaUnlockWithinHoursFromGameStart int32 = 3
	GachaUnlockMainQuestClear           int32 = 4
	GachaUnlockMainFunctionReleased     int32 = 5
)

const (
	GachaAutoResetUnknown int32 = 0
	GachaAutoResetNone    int32 = 1
	GachaAutoResetDaily   int32 = 2
	GachaAutoResetMonthly int32 = 3
)

const (
	GachaDecorationUnknown  int32 = 0
	GachaDecorationNormal   int32 = 1
	GachaDecorationFestival int32 = 2
)

const (
	GachaBadgeTypeNone int32 = 1
)

const (
	PriceTypeUnknown         int32 = 0
	PriceTypeConsumableItem  int32 = 1
	PriceTypeGem             int32 = 2
	PriceTypePaidGem         int32 = 3
	PriceTypePlatformPayment int32 = 4
)

const (
	BannerPrefixLimited = "limited_"
	BannerPrefixStepUp  = "step_up_"
	BannerPrefixCommon  = "common_"
)

func IsMaterialBanner(labelType int32) bool {
	return labelType == GachaLabelChapter || labelType == GachaLabelRecycle || labelType == GachaLabelPortalCage
}

const MomBannerDomainGacha int32 = 1

const StepUpGroupDivisor int32 = 1000

// The values in this block are the compiled-in defaults for the settings
// documented in internal/tunables: an operator's tunables.json overrides them at
// startup, and nothing else writes to them. They are variables rather than
// constants for that reason.
var (
	PityCeilingCount int32 = 200

	PremiumSinglePullPrice int32 = 300
	PremiumMultiPullPrice  int32 = 3000
	PremiumMultiPullCount  int32 = 10

	StepUpStep1Cost int32 = 2000
	StepUpStep3Cost int32 = 3000
	StepUpStep5Cost int32 = 5000

	FeaturedRateUpPercent int = 35

	StepUpRateBoost    float64 = 1.5
	StepUpRateMaxBoost float64 = 2.0

	DupGradeMin   int32 = 2
	DupGradeRange int   = 4

	DefaultDailyDrawLimit int32 = 5

	BoxPoolMaxItems    int   = 50
	BoxPoolMinItems    int   = 5
	BoxItemDefaultMax  int32 = 10
	BoxFallbackItemMax int32 = 20
)

// Not knobs: these are semantics or content rather than tuning.
const (
	MedalCountCap int32 = 9999

	// StepUpFreeCost is the price of the second and fourth step-up steps.
	StepUpFreeCost int32 = 0

	// FeaturedRateUpDenom is the denominator FeaturedRateUpPercent is out of; it
	// stays fixed so the percentage in a tunables file means what it says.
	FeaturedRateUpDenom int = 100

	// BoxFallbackItemId is granted when a box gacha has too few entries.
	BoxFallbackItemId int32 = 100001
)

type DupExchangeEntry struct {
	PossessionType int32
	PossessionId   int32
	Count          int32
}

const PhaseIdMultiplier int32 = 10

const (
	ConsumableIdPremiumTicket int32 = 1
	ConsumableIdChapterTicket int32 = 2
)
