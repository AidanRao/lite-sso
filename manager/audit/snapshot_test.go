package audit

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sso-server/model"
	"testing"
	"time"
)

func Test_Snapshot_FeaturePolicyMetadata(t *testing.T) {
	event, ok := snapshot(model.AuditLog{ID: uuid.NewString(), OccurredAt: time.Now(), Outcome: "success", Action: "admin.feature.update", TargetType: "feature", TargetID: "profile.audit_logs", Details: `{"changed_fields":["audience","percentage","stage","user_ids","secret"],"user_ids":["private-user"]}`})
	require.True(t, ok)
	require.Equal(t, "profile.audit_logs", event.TargetID)
	require.NotContains(t, event.Details, "private-user")
	require.NotContains(t, event.Details, "secret")
	var details model.AuditDetails
	require.NoError(t, json.Unmarshal([]byte(event.Details), &details))
	require.Equal(t, []string{"audience", "percentage", "stage", "user_ids"}, details.ChangedFields)
}
