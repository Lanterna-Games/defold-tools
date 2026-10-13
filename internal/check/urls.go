package check

import (
	"fmt"
	"slices"

	"github.com/Lanterna-Games/defold-tools/internal/engine"
	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/project"
	"github.com/Lanterna-Games/defold-tools/internal/suggest"
)

const (
	unresolvedURLCheck = "unresolved-url"
	existenceProbe     = "go.exists"
)

type urlUse struct {
	text     string
	location project.Location
	subject  string
}

type urlVerdicts struct {
	judged   map[project.Location]int
	failures map[project.Location][]string
}

func unresolvedURLs(inspected inspection) []Problem {
	loaded := inspected.project
	verdicts := urlVerdicts{judged: map[project.Location]int{}, failures: map[project.Location][]string{}}

	owners := loaded.Owners()
	for _, mount := range loaded.Mounts() {
		for _, instance := range mount.Collection.Instances {
			owner := project.Owner{
				Namespace:    mount.Namespace,
				InstancePath: mount.Prefix + "/" + instance.ID,
				BasePrefix:   mount.Prefix,
				GameObject:   loaded.GameObjectOf(instance),
			}
			verdicts.judgeAll(inspected, owner, urlPropertiesIn(owner.InstancePath, instance.PropertyOverrides))
		}
		for _, subCollection := range mount.Collection.SubCollections {
			nestedPrefix := mount.Prefix + "/" + subCollection.ID
			for instanceID, overrides := range subCollection.PropertyOverrides {
				instancePath := nestedPrefix + "/" + instanceID
				owner := project.Owner{
					Namespace:    mount.Namespace,
					InstancePath: instancePath,
					BasePrefix:   nestedPrefix,
					GameObject:   mount.Namespace.GameObjects[instancePath],
				}
				verdicts.judgeAll(inspected, owner, urlPropertiesIn(instancePath, overrides))
			}
		}
	}
	addressesIn := map[string][]urlUse{}
	for _, owner := range owners {
		for _, scriptPath := range loaded.ScriptPathsOf(owner.GameObject) {
			script, isLoaded := loaded.Scripts[scriptPath]
			if !isLoaded {
				continue
			}
			if _, isRead := addressesIn[scriptPath]; !isRead {
				addressesIn[scriptPath] = addressesWrittenIn(script, inspected.reference)
			}
			verdicts.judgeAll(inspected, owner, addressesIn[scriptPath])
		}
	}
	return verdicts.problems()
}

func urlPropertiesIn(instancePath string, overrides []project.PropertyOverride) []urlUse {
	var uses []urlUse
	for _, override := range overrides {
		if override.Type != project.PropertyTypeURL || override.Value == "" {
			continue
		}
		uses = append(uses, urlUse{
			text:     override.Value,
			location: override.Location,
			subject:  fmt.Sprintf("property %q of %q points at %q, but", override.Property, instancePath+"#"+override.Component, override.Value),
		})
	}
	return uses
}

func addressesWrittenIn(script *project.Script, reference *engine.Reference) []urlUse {
	var uses []urlUse
	isAskedWhetherItExists := map[string]bool{}
	use := func(address project.Address) {
		uses = append(uses, urlUse{text: address.Text, location: address.Location, subject: fmt.Sprintf("%q:", address.Text)})
	}
	for _, address := range script.URLsBuilt() {
		use(address)
	}
	lua.Inspect(script.Chunk, func(node lua.Node) bool {
		call, isCall := node.(*lua.CallExpression)
		if !isCall || call.Method != nil {
			return true
		}
		function, isQualifiedName := lua.QualifiedName(call.Function)
		if !isQualifiedName {
			return true
		}
		for _, index := range reference.URLParameterIndexes(function) {
			if index >= len(call.Arguments) {
				continue
			}
			address, isWritten := script.AddressWrittenIn(call.Arguments[index])
			switch {
			case !isWritten:
			case function == existenceProbe:
				isAskedWhetherItExists[address.Text] = true
			default:
				use(address)
			}
		}
		return true
	})
	return slices.DeleteFunc(uses, func(use urlUse) bool { return isAskedWhetherItExists[use.text] })
}

func (v urlVerdicts) judgeAll(inspected inspection, owner project.Owner, uses []urlUse) {
	for _, use := range uses {
		v.judged[use.location]++
		if reason, isBroken := inspected.whyURLDoesNotResolve(owner, use.text); isBroken {
			v.failures[use.location] = append(v.failures[use.location], use.subject+" "+reason)
		}
	}
}

func (v urlVerdicts) problems() []Problem {
	var problems []Problem
	for location, failures := range v.failures {
		isBrokenWhereverItIsUsed := len(failures) == v.judged[location]
		if isBrokenWhereverItIsUsed {
			problems = append(problems, Problem{Check: unresolvedURLCheck, Location: location, Message: failures[0]})
		}
	}
	return problems
}

func (i inspection) whyURLDoesNotResolve(owner project.Owner, text string) (reason string, isBroken bool) {
	url := project.ParseURL(text)
	if url.PointsAtItsOwnGameObject() {
		return whyComponentIsMissing(owner.InstancePath, owner.GameObject, url)
	}
	namespaces := i.project.NamespacesAddressedBy(owner, url)
	for _, namespace := range namespaces {
		whyNotHere, isBrokenHere := whyNamespaceDoesNotHave(namespace, owner.PathAddressedBy(url), url)
		if !isBrokenHere {
			return "", false
		}
		if reason == "" {
			reason = whyNotHere
		}
	}
	return reason, len(namespaces) > 0
}

func whyNamespaceDoesNotHave(namespace *project.Namespace, targetPath string, url project.URL) (reason string, isBroken bool) {
	gameObject, exists := namespace.GameObjects[targetPath]
	if !exists {
		if namespace.CouldHoldMoreThanWasRead(targetPath) {
			return "", false
		}
		reason = fmt.Sprintf("collection %q has no game object %q", namespace.Root.Name, targetPath)
		if closest, found := suggest.Closest(targetPath, namespace.GameObjectPaths()); found {
			reason += fmt.Sprintf(" (did you mean %q?)", closest)
		}
		return reason, true
	}
	return whyComponentIsMissing(targetPath, gameObject, url)
}

func whyComponentIsMissing(gameObjectName string, gameObject *project.GameObject, url project.URL) (reason string, isBroken bool) {
	couldNotBeRead := gameObject == nil
	if couldNotBeRead || !url.HasFragment || url.Fragment == "" {
		return "", false
	}
	if _, hasComponent := gameObject.Component(url.Fragment); hasComponent {
		return "", false
	}
	reason = fmt.Sprintf("%q has no component %q", gameObjectName, url.Fragment)
	if closest, found := suggest.Closest(url.Fragment, gameObject.ComponentIDs()); found {
		reason += fmt.Sprintf(" (did you mean %q?)", closest)
	}
	return reason, true
}
