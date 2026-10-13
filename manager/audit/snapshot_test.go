package audit

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sso-server/manager/thirdparty"
	"sso-server/model"
	"testing"
	"time"
)

func Test_Snapshot_FeaturePolicyMetadata(t *testing.T) {
	event, ok := snapshot(model.AuditLog{ID: uuid.NewString(), OccurredAt: time.Now(), Outcome: "success", Action: "admin.feature.update", TargetType: "feature", TargetID: "profile.audit_logs", Details: `{"changed_fields":["audience","percentage","stage","user_ids","secret"],"user_ids":["private-user"]}`}, thirdparty.NewBuiltins(nil))
	require.True(t, ok)
	require.Equal(t, "profile.audit_logs", event.TargetID)
	require.NotContains(t, event.Details, "private-user")
	require.NotContains(t, event.Details, "secret")
	var details model.AuditDetails
	require.NoError(t, json.Unmarshal([]byte(event.Details), &details))
	require.Equal(t, []string{"audience", "percentage", "stage", "user_ids"}, details.ChangedFields)
}

func Test_Snapshot_ThirdPartyProviderMetadata(t *testing.T) {
	for _, test := range []struct {
		name     string
		details  string
		provider string
	}{
		{name: "qq", details: `{"provider":"qq","auth_method":"qq"}`, provider: "qq"},
		{name: "unknown", details: `{"provider":"unknown","auth_method":"unknown"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			event, ok := snapshot(model.AuditLog{ID: uuid.NewString(), OccurredAt: time.Now(), Outcome: "success", Action: "auth.login.third_party", Details: test.details}, thirdparty.NewBuiltins(nil))
			require.True(t, ok)
			var details model.AuditDetails
			require.NoError(t, json.Unmarshal([]byte(event.Details), &details))
			require.Equal(t, test.provider, details.Provider)
			require.Equal(t, test.provider, details.AuthMethod)
		})
	}
}
