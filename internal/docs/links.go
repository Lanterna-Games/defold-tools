package docs

import "fmt"

const (
	referenceOnTheWeb     = "https://defold.com/ref/%s-lua#%s"
	anchorOfEveryMessage  = "message-"
	anchorOfEveryProperty = "property-"
)

type Links struct {
	OfflineReference string
}

func (l Links) To(version, namespace, anchor, anchorOnTheWeb string) string {
	onTheWeb := fmt.Sprintf("[defold.com]("+referenceOnTheWeb+")", namespace, anchorOnTheWeb)
	if l.OfflineReference == "" {
		return onTheWeb
	}
	return fmt.Sprintf("[Offline reference](%s/%s/%s#%s) · %s", l.OfflineReference, version, namespace, anchor, onTheWeb)
}

func (l Links) ToTheMessage(version, namespace, name string) string {
	return l.To(version, namespace, AnchorOfTheMessage(name), name)
}

func (l Links) ToTheProperty(version, namespace, name string) string {
	return l.To(version, namespace, AnchorOfTheProperty(name), name)
}

func AnchorOfTheMessage(name string) string {
	return anchorOfEveryMessage + name
}

func AnchorOfTheProperty(name string) string {
	return anchorOfEveryProperty + name
}
