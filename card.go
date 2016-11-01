package mtg

// TODO: add support for analyzing deck statistics, generating random hands, etc.

import (
	"container/list"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var (
	cardCache = make(map[string]*Card)
)

// Card represents a Magic card.
type Card struct {
	// MultiverseID is the "multiverseid" value used by Gatherer.
	MultiverseID int
	// Name is the name of the card.
	Name string
	// ManaCost is the mana cost of the card. It is defined as a slice,
	// where each element is a component of the cost. For example, a
	// card that costs 2UU would be represented as []string{"2", "blue", "blue"}.
	ManaCost []string
	// ConvertedManaCost is the total converted mana cost of the card.
	ConvertedManaCost int
	// Type is the type of the card.
	Type string
	// Text is the text of the card.
	Text string
	// Rarity is the rarity of the card.
	Rarity string
}

// GetCard retrieves card information from Gatherer given a multiverseid.
func FetchCard(multiverseid int) (*Card, error) {
	resp, err := http.Get(fmt.Sprintf("http://gatherer.wizards.com/Pages/Card/Details.aspx?multiverseid=%d", multiverseid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	card, err := parseCard(resp.Body)
	if err != nil {
		return nil, err
	}

	card.MultiverseID = multiverseid
	return card, nil
}

// GetCardForName searches Gatherer for the given card. Errors are only
// returned when a network  or unexpected error occurs; both return values
// will be nil if the card was simply not found. An internal cache is used
// to speed up subsequent calls for the same name.
func GetCardForName(name string) (*Card, error) {
	if card, ok := cardCache[name]; ok {
		return card, nil
	}

	u, err := url.Parse("http://gatherer.wizards.com/Pages/Search/Default.aspx")
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Add("name", name)

	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.Request.URL.Path == "/Pages/Error.aspx" {
		return nil, nil
	}

	card, err := parseCard(resp.Body)
	if err != nil {
		return nil, err
	}

	if multiverseid := resp.Request.URL.Query().Get("multiverseid"); multiverseid != "" {
		card.MultiverseID, err = strconv.Atoi(multiverseid)
	}

	cardCache[name] = card
	return card, err
}

// ClearCardCache clears the internal cache used by GetCardForName.
func ClearCardCache() {
	cardCache = make(map[string]*Card)
	runtime.GC()
}

func parseCard(r io.Reader) (*Card, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	cardDetailsTable := findNode(doc, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "table" && nodeHasClass(node, "cardDetails")
	})

	if cardDetailsTable == nil {
		return nil, nil
	}

	var (
		card        = &Card{}
		getRowValue = func(node *html.Node) *html.Node {
			return findNode(node, func(node *html.Node) bool {
				return nodeHasClass(node, "value")
			})
		}
	)

	var (
		nameRow = findNode(cardDetailsTable, nodeIdHasSuffix("_nameRow"))
		manaRow = findNode(cardDetailsTable, nodeIdHasSuffix("_manaRow"))
		cmcRow  = findNode(cardDetailsTable, nodeIdHasSuffix("_cmcRow"))
		typeRow = findNode(cardDetailsTable, nodeIdHasSuffix("_typeRow"))
		textRow = findNode(cardDetailsTable, nodeIdHasSuffix("_textRow"))
		// setRow       = findNode(cardDetailsTable, nodeIdHasSuffix("_setRow"))
		// rarityRow    = findNode(cardDetailsTable, nodeIdHasSuffix("_rarityRow"))
		// otherSetsRow = findNode(cardDetailsTable, nodeIdHasSuffix("_otherSetsRow"))
		// numberRow    = findNode(cardDetailsTable, nodeIdHasSuffix("_numberRow"))
		// artistRow    = findNode(cardDetailsTable, nodeIdHasSuffix("_artistRow"))
	)

	card.Name = strings.TrimSpace(getRowValue(nameRow).FirstChild.Data)
	for c := getRowValue(manaRow).FirstChild.NextSibling; c != nil; c = c.NextSibling {
		card.ManaCost = append(card.ManaCost, strings.ToLower(getAttr(c.Attr, "alt")))
	}
	if cmc, err := strconv.Atoi(getRowValue(cmcRow).FirstChild.Data); err == nil {
		card.ConvertedManaCost = cmc
	}
	card.Type = strings.TrimSpace(getRowValue(typeRow).FirstChild.Data)
	card.Text = strings.TrimSpace(getRowValue(textRow).FirstChild.NextSibling.FirstChild.Data)

	return card, nil
}

func findNode(root *html.Node, f func(*html.Node) bool) *html.Node {
	queue := list.New()
	queue.PushBack(root)
	for queue.Len() != 0 {
		node := queue.Remove(queue.Front()).(*html.Node)
		if f(node) {
			return node
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			queue.PushBack(child)
		}
	}
	return nil
}

func nodeHasClass(node *html.Node, class string) bool {
	for _, c := range strings.Fields(getAttr(node.Attr, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func nodeIdHasSuffix(suffix string) func(*html.Node) bool {
	return func(node *html.Node) bool {
		return strings.HasSuffix(getAttr(node.Attr, "id"), suffix)
	}
}

func getAttr(attrs []html.Attribute, name string) string {
	for _, attr := range attrs {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func walkNode(node *html.Node) {
	var f func(*html.Node, int)
	f = func(node *html.Node, depth int) {
		if node == nil {
			return
		}
		indent := strings.Repeat("  ", depth)
		for ; node != nil; node = node.NextSibling {
			fmt.Printf("%s[%d] %s\n", indent, node.Type, strings.TrimSpace(node.Data))
			f(node.FirstChild, depth+1)
		}
	}
	f(node, 0)
}
