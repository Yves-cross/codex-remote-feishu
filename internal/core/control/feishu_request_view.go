package control

// FeishuRequestView is the UI-owned request payload used by the Feishu adapter
// for approval / request_user_input / permissions / elicitation cards.
type FeishuRequestView struct {
	RequestID                          string
	RequestType                        string
	RequestRevision                    int
	Title                              string
	ThreadID                           string
	ThreadTitle                        string
	Sections                           []FeishuCardTextSection
	Options                            []RequestPromptOption
	Questions                          []RequestPromptQuestion
	CurrentQuestionIndex               int
	SubmitWithUnansweredConfirmPending bool
	SubmitWithUnansweredMissingLabels  []string
}
