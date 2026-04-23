package daemon

import frontstagecontract "github.com/kxn/codex-remote-feishu/internal/core/frontstagecontract"

const (
	upgradeOwnerPayloadKind      = frontstagecontract.CardActionKindUpgradeOwnerFlow
	upgradeOwnerPayloadFlowKey   = frontstagecontract.CardActionPayloadKeyPickerID
	upgradeOwnerPayloadOptionKey = frontstagecontract.CardActionPayloadKeyOptionID

	vscodeMigrationOwnerPayloadKind    = frontstagecontract.CardActionKindVSCodeMigrateOwnerFlow
	vscodeMigrationOwnerPayloadFlowKey = frontstagecontract.CardActionPayloadKeyPickerID
	vscodeMigrationOwnerPayloadRunKey  = frontstagecontract.CardActionPayloadKeyOptionID
)
