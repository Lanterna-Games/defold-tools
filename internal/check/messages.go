package check

import (
	"fmt"
	"path"
	"sort"

	"github.com/Lanterna-Games/defold-tools/internal/project"
)

const (
	unhandledMessageCheck = "unhandled-message"
	unpostedMessageCheck  = "unposted-message"
	renderScriptExtension = ".render_script"
)

type messageTraffic struct {
	handledSomewhere       map[string]bool
	postedSomewhere        map[string]bool
	someHandlerIsOpenEnded bool
	somePostHasAnUnknownID bool
}

func trafficOf(loaded *project.Project) messageTraffic {
	traffic := messageTraffic{handledSomewhere: map[string]bool{}, postedSomewhere: map[string]bool{}}
	for _, script := range loaded.Scripts {
		if handlesWhatItDoesNotName(script) {
			traffic.someHandlerIsOpenEnded = true
		}
		if !script.IsReadInFull {
			traffic.somePostHasAnUnknownID = true
		}
		for _, handled := range script.Handled {
			traffic.handledSomewhere[handled.Name] = true
		}
		for _, post := range script.Posts {
			if post.IDIsKnown {
				traffic.postedSomewhere[post.ID.Name] = true
			} else {
				traffic.somePostHasAnUnknownID = true
			}
		}
	}
	return traffic
}

func handlesWhatItDoesNotName(script *project.Script) bool {
	return script.HandlesMessagesItDoesNotName || !script.IsReadInFull
}

func namesOf(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type messageJudge struct {
	inspection
	traffic  messageTraffic
	verdicts verdicts
}

func unhandledMessages(inspected inspection) []Problem {
	loaded := inspected.project
	judge := messageJudge{
		inspection: inspected,
		traffic:    trafficOf(loaded),
		verdicts:   newVerdicts(unhandledMessageCheck),
	}

	hasAnOwner := map[string]bool{}
	for _, owner := range loaded.Owners() {
		for _, scriptPath := range loaded.ScriptPathsOf(owner.GameObject) {
			script, isLoaded := loaded.Scripts[scriptPath]
			if !isLoaded {
				continue
			}
			hasAnOwner[scriptPath] = true
			for _, post := range script.Posts {
				judge.judge(post, script, &owner)
			}
		}
	}
	for _, scriptPath := range loaded.ScriptPaths() {
		if hasAnOwner[scriptPath] {
			continue
		}
		for _, post := range loaded.Scripts[scriptPath].Posts {
			judge.judge(post, loaded.Scripts[scriptPath], nil)
		}
	}
	return judge.verdicts.problems()
}

func (j messageJudge) judge(post project.MessagePost, sender *project.Script, owner *project.Owner) {
	if !post.IDIsKnown || j.reference.HasMessage(post.ID.Name) {
		return
	}
	url := project.ParseURL(post.Receiver)
	belongsToNoGameObject := owner == nil
	if !post.ReceiverIsWritten || (belongsToNoGameObject && url.Socket == "") {
		j.judgeAgainstTheWholeProject(post)
		return
	}
	if belongsToNoGameObject {
		owner = &project.Owner{}
	}

	receiver, isInSight := j.receiverOf(url, sender, *owner)
	if !isInSight {
		j.verdicts.accept(post.ID.Location)
		return
	}
	var handledByTheReceiver []string
	for _, script := range receiver.scripts {
		if handlesWhatItDoesNotName(script) || script.Handles(post.ID.Name) {
			j.verdicts.accept(post.ID.Location)
			return
		}
		handledByTheReceiver = append(handledByTheReceiver, script.HandledNames()...)
	}
	rejection := fmt.Sprintf("%q is posted to %q, but %s", post.ID.Name, post.Receiver, receiver.whyItDoesNotHandle())
	if hint := j.hintTowardsAMessage(post.ID.Name, handledByTheReceiver); hint != "" {
		rejection += hint
	} else if len(handledByTheReceiver) > 0 {
		rejection += "; it handles " + quotedList(handledByTheReceiver)
	}
	j.verdicts.reject(post.ID.Location, rejection)
}

func (j messageJudge) judgeAgainstTheWholeProject(post project.MessagePost) {
	someoneCouldHandleIt := j.traffic.handledSomewhere[post.ID.Name] || j.traffic.someHandlerIsOpenEnded || j.project.RunsCodeOutOfSight
	if someoneCouldHandleIt {
		j.verdicts.accept(post.ID.Location)
		return
	}
	j.verdicts.reject(post.ID.Location, fmt.Sprintf("%q is posted here, but no script in the project handles it%s",
		post.ID.Name, j.hintTowardsAMessage(post.ID.Name, namesOf(j.traffic.handledSomewhere))))
}

func (i inspection) hintTowardsAMessage(name string, ofTheProject []string) string {
	return hintTowards(name, append(append([]string{}, ofTheProject...), i.reference.Messages...), "%q")
}

type messageReceiver struct {
	name          string
	scripts       []*project.Script
	soleComponent *project.Component
}

func (r messageReceiver) whyItDoesNotHandle() string {
	switch {
	case len(r.scripts) == 1:
		return r.scripts[0].Path + " does not handle it"
	case r.soleComponent != nil:
		return fmt.Sprintf("it is a %s component, which takes engine messages only", r.soleComponent.Type)
	default:
		return fmt.Sprintf("no component of %q handles it", r.name)
	}
}

func (j messageJudge) receiverOf(url project.URL, sender *project.Script, owner project.Owner) (receiver messageReceiver, isInSight bool) {
	addressesTheSenderItself := url.Socket == "" && url.Path == "" && url.HasFragment && url.Fragment == ""
	if addressesTheSenderItself {
		return messageReceiver{scripts: []*project.Script{sender}}, true
	}
	target, isKnown := j.project.TargetOf(owner, url)
	if !isKnown {
		return messageReceiver{}, false
	}

	receiver.name = target.Name
	components := target.GameObject.Components
	if url.HasFragment {
		component, exists := target.GameObject.Component(url.Fragment)
		if !exists {
			return messageReceiver{}, false
		}
		components, receiver.soleComponent = []project.Component{component}, &component
	}
	for _, component := range components {
		script, scriptIsInSight := j.project.ScriptRunBy(component)
		runsCodeOfItsOwn := component.IsScript() || script != nil
		if !scriptIsInSight || (!runsCodeOfItsOwn && j.project.RunsCodeOutOfSight) {
			return messageReceiver{}, false
		}
		if script != nil {
			receiver.scripts = append(receiver.scripts, script)
		}
	}
	return receiver, true
}

func unpostedMessages(inspected inspection) []Problem {
	loaded := inspected.project
	traffic := trafficOf(loaded)
	if loaded.RunsCodeOutOfSight || traffic.somePostHasAnUnknownID {
		return nil
	}

	var problems []Problem
	for _, scriptPath := range loaded.ScriptPaths() {
		if path.Ext(scriptPath) == renderScriptExtension {
			continue
		}
		for _, handled := range loaded.Scripts[scriptPath].Handled {
			if traffic.postedSomewhere[handled.Name] || inspected.reference.HasMessage(handled.Name) {
				continue
			}
			problems = append(problems, Problem{
				Check:    unpostedMessageCheck,
				Location: handled.Location,
				Message: fmt.Sprintf("%q is handled here, but nothing in the project posts it%s",
					handled.Name, inspected.hintTowardsAMessage(handled.Name, namesOf(traffic.postedSomewhere))),
			})
		}
	}
	return problems
}
