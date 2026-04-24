package eventcontract

import "strings"

type AttentionAnnotation struct {
	Text          string
	MentionUserID string
}

func (annotation AttentionAnnotation) Normalized() AttentionAnnotation {
	annotation.Text = strings.TrimSpace(annotation.Text)
	annotation.MentionUserID = strings.TrimSpace(annotation.MentionUserID)
	if annotation.Text == "" || annotation.MentionUserID == "" {
		return AttentionAnnotation{}
	}
	return annotation
}

func (annotation AttentionAnnotation) Empty() bool {
	return annotation.Normalized() == (AttentionAnnotation{})
}
