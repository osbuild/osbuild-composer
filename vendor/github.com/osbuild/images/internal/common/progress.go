package common

import (
	"fmt"
	"strings"
)

type Progress struct {
	Name        string    `json:"name"`
	Total       int       `json:"total"`
	Done        int       `json:"done"`
	SubProgress *Progress `json:"progress,omitempty"`
}

type ProgressWrapper struct {
	Message  string    `json:"message"`
	Progress *Progress `json:"progress,omitempty"`
}

func (p Progress) ToShortString() (string, float64) {
	appendix := ""
	subProgressVal := float64(0)
	if p.SubProgress != nil {
		var subString string
		subString, subProgressVal = p.SubProgress.ToShortString()
		appendix = " -> " + subString
	}
	retString := fmt.Sprintf("\"%v\" (%v/%v)%v", p.Name, p.Done, p.Total, appendix)
	retProgressVal := float64(0)
	if p.Total != 0 {
		retProgressVal = float64(p.Done) / float64(p.Total)
	}

	retProgressVal += subProgressVal * (1 / float64(p.Total))
	return retString, retProgressVal
}

func (p ProgressWrapper) ToShortString() string {
	ret, retProgress := p.Progress.ToShortString()
	if len(p.Message) > 0 {
		ret += " -> \"" + strings.TrimSuffix(p.Message, "\n") + "\""
	}
	ret = fmt.Sprintf("%d%% %s", int(retProgress*100), ret)
	return ret
}
