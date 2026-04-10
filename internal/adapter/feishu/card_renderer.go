package feishu

import "strings"

type cardEnvelopeVersion string

const (
	cardEnvelopeLegacy cardEnvelopeVersion = "legacy"
	cardEnvelopeV2     cardEnvelopeVersion = "v2"
)

type cardDocument struct {
	Title      string
	ThemeKey   string
	Components []cardComponent
}

type cardComponent interface {
	renderCardComponent(version cardEnvelopeVersion) map[string]any
}

type cardMarkdownComponent struct {
	Content string
}

type cardRawComponent struct {
	legacy map[string]any
	v2     map[string]any
}

func newCardDocument(title, themeKey string, components ...cardComponent) *cardDocument {
	doc := &cardDocument{
		Title:      strings.TrimSpace(title),
		ThemeKey:   strings.TrimSpace(themeKey),
		Components: make([]cardComponent, 0, len(components)),
	}
	for _, component := range components {
		if component == nil {
			continue
		}
		doc.Components = append(doc.Components, component)
	}
	return doc
}

func legacyCardDocument(title, body, themeKey string, extraElements []map[string]any) *cardDocument {
	components := make([]cardComponent, 0, len(extraElements)+1)
	if strings.TrimSpace(body) != "" {
		components = append(components, cardMarkdownComponent{Content: body})
	}
	for _, element := range extraElements {
		components = append(components, newRawCardComponent(element))
	}
	return newCardDocument(title, themeKey, components...)
}

func newRawCardComponent(data map[string]any) cardComponent {
	return cardRawComponent{
		legacy: cloneCardMap(data),
		v2:     cloneCardMap(data),
	}
}

func (c cardMarkdownComponent) renderCardComponent(_ cardEnvelopeVersion) map[string]any {
	if strings.TrimSpace(c.Content) == "" {
		return nil
	}
	return map[string]any{
		"tag":     "markdown",
		"content": c.Content,
	}
}

func (c cardRawComponent) renderCardComponent(version cardEnvelopeVersion) map[string]any {
	if version == cardEnvelopeV2 && len(c.v2) != 0 {
		return cloneCardMap(c.v2)
	}
	return cloneCardMap(c.legacy)
}

func renderOperationCard(operation Operation, version cardEnvelopeVersion) map[string]any {
	doc := operation.card
	if doc == nil {
		doc = legacyCardDocument(operation.CardTitle, operation.CardBody, operation.CardThemeKey, operation.CardElements)
	}
	if doc == nil {
		return nil
	}
	return renderCardDocument(doc, version)
}

func (operation Operation) ordinaryCardEnvelope() cardEnvelopeVersion {
	if operation.cardEnvelope == cardEnvelopeV2 {
		return cardEnvelopeV2
	}
	return cardEnvelopeLegacy
}

func renderCardDocument(doc *cardDocument, version cardEnvelopeVersion) map[string]any {
	if doc == nil {
		return nil
	}
	elements := make([]map[string]any, 0, len(doc.Components))
	for _, component := range doc.Components {
		if component == nil {
			continue
		}
		rendered := component.renderCardComponent(version)
		if len(rendered) == 0 {
			continue
		}
		elements = append(elements, rendered)
	}
	header := map[string]any{
		"template": cardTemplate(doc.ThemeKey, doc.Title),
		"title": map[string]any{
			"tag":     "plain_text",
			"content": doc.Title,
		},
	}
	switch version {
	case cardEnvelopeV2:
		return map[string]any{
			"schema": "2.0",
			"config": map[string]any{
				"width_mode":     "fill",
				"enable_forward": true,
			},
			"header": header,
			"body": map[string]any{
				"elements": elements,
			},
		}
	default:
		return map[string]any{
			"config": map[string]any{
				"wide_screen_mode": true,
				"enable_forward":   true,
			},
			"header":   header,
			"elements": elements,
		}
	}
}

func cloneCardMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, raw := range value {
		out[key] = cloneCardAny(raw)
	}
	return out
}

func cloneCardAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneCardMap(typed)
	case []map[string]any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneCardMap(item))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneCardAny(item))
		}
		return out
	default:
		return typed
	}
}
