package auth

type Action string

const (
	ActionRead   Action = "read"
	ActionWrite  Action = "write"
	ActionDelete Action = "delete"
	ActionList   Action = "list"
)

type Decision struct {
	Allowed bool
	Reason  string
}

type AuthorizationRequest struct {
	Claims    Claims
	BucketName string
	Action    Action
	ObjectKey string
}

const (
	DecisionReasonBucketScope  = "bucket_scope"
	DecisionReasonActionScope  = "action_scope"
	DecisionReasonPrefixScope  = "prefix_scope"
	DecisionReasonInvalidInput = "invalid_input"
)

func (a Action) Valid() bool {
	switch a {
	case ActionRead, ActionWrite, ActionDelete, ActionList:
		return true
	default:
		return false
	}
}
