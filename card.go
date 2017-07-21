package mtg

// TODO: add support for analyzing deck statistics, generating random hands, etc.

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

const gathererBase = "http://gatherer.wizards.com"

var (
	cardCache = make(map[string]Card)
	allColors = []string{"W", "U", "B", "R", "G"}
)

// Card represents a Magic card.
type Card struct {
	// MultiverseID is the "multiverseid" value used by Gatherer.
	MultiverseID int
	// Name is the name of the card.
	Name string
	// ManaCost is the mana cost of the card.
	ManaCost string
	// ConvertedManaCost is the total converted mana cost of the card.
	ConvertedManaCost int
	// Type is the type of the card.
	Type string
	// Text is the text of the card.
	Text string
	// Rarity is the rarity of the card.
	Rarity string
}

func (c Card) Colors() (colors []string) {
	for _, color := range allColors {
		if strings.Contains(c.ManaCost, color) {
			colors = append(colors, color)
		}
	}
	return colors
}

// GetCard retrieves card information from Gatherer given a multiverseid.
func FetchCard(multiverseid int) (Card, error) {
	resp, err := http.Get(fmt.Sprintf(gathererBase+"/Pages/Card/Details.aspx?multiverseid=%d", multiverseid))
	if err != nil {
		return Card{}, err
	}
	defer resp.Body.Close()

	card, err := parseCard(resp.Body)
	if err != nil {
		return Card{}, err
	}

	card.MultiverseID = multiverseid
	return card, nil
}

// GetCardForName searches Gatherer for the given card. Errors are only
// returned when a network  or unexpected error occurs; both return values
// will be nil if the card was simply not found. An internal cache is used
// to speed up subsequent calls for the same name.
func GetCardForName(name string) (Card, error) {
	if card, ok := cardCache[name]; ok {
		return card, nil
	}

	page, err := makeGathererRequest("", name)
	if err != nil {
		return Card{}, err
	}
	defer page.Body.Close()

	var buf bytes.Buffer
	io.Copy(&buf, page.Body)
	// fmt.Println(buf.String())

	card, err := parseCard(&buf)
	if err != nil {
		return Card{}, err
	}

	if multiverseid := page.Request.URL.Query().Get("multiverseid"); multiverseid != "" {
		card.MultiverseID, err = strconv.Atoi(multiverseid)
	}

	cardCache[name] = card
	return card, err
}

// ClearCardCache clears the internal cache used by GetCardForName.
func ClearCardCache() {
	cardCache = make(map[string]Card)
	runtime.GC()
}

func makeGathererRequest(reqURL, cardName string) (*http.Response, error) {
	if reqURL == "" {
		var buf bytes.Buffer
		for _, part := range strings.Fields(cardName) {
			buf.WriteString("+[" + part + "]")
		}
		query := url.Values{}
		query.Add("name", buf.String())
		reqURL = gathererBase + "/Pages/Search/Default.aspx?" + query.Encode()
	}
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, errors.New("makeGathererRequest: " + err.Error())
	}
	switch resp.Request.URL.Path {
	case "/Pages/Card/Details.aspx":
		return resp, nil
	case "/Pages/Error.aspx":
		resp.Body.Close()
		return nil, errors.New("makeGathererRequest: search redirected to Error.aspx")
	case "/Pages/Search/Default.aspx":
		doc, err := html.Parse(resp.Body)
		if err != nil {
			return nil, err
		}
		tableNode := findNode(doc, func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "table" && nodeHasClass(node, "cardItemTable")
		})
		if tableNode == nil {
			return nil, errors.New("no results found; perhaps you misspelled it?")
		}
		cardItems := findAllNodes(tableNode, func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "tr" && nodeHasClass(node, "cardItem")
		})
		if len(cardItems) == 0 {
			return nil, errors.New("no cards found in table")
		}
		for _, cardItem := range cardItems {
			titleNode := findNode(cardItem, func(node *html.Node) bool {
				return node.Type == html.ElementNode && node.Data == "span" && nodeHasClass(node, "cardTitle")
			})
			if titleNode == nil {
				continue
			}
			if titleNode.FirstChild.NextSibling.FirstChild.Data == cardName {
				cardUrl := getAttr(titleNode.FirstChild.NextSibling.Attr, "href")
				return makeGathererRequest(gathererBase+resolvePath(resp.Request.URL.Path, cardUrl), cardName)
			}
		}
		return nil, errors.New("card " + cardName + " not found on search result page")
	default:
		return nil, errors.New("makeGathererRequest: unknown url path: " + resp.Request.URL.Path)
	}
}

func parseCard(r io.Reader) (Card, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return Card{}, err
	}

	cardDetailsTable := findNode(doc, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "table" && nodeHasClass(node, "cardDetails")
	})

	if cardDetailsTable == nil {
		return Card{}, errors.New("no cardDetails table found")
	}

	var (
		card        = Card{}
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
	if manaRow != nil {
		for c := getRowValue(manaRow).FirstChild.NextSibling; c != nil; c = c.NextSibling {
			part := getAttr(c.Attr, "alt")
			if _, err := strconv.Atoi(part); err == nil {
				card.ManaCost += part
			} else {
				switch strings.ToUpper(part) {
				case "WHITE":
					card.ManaCost += "W"
				case "BLUE":
					card.ManaCost += "U"
				case "BLACK":
					card.ManaCost += "B"
				case "RED":
					card.ManaCost += "R"
				case "GREEN":
					card.ManaCost += "G"
				default:
					fmt.Println("unknown mana cost part: " + part)
				}
			}
		}
	}
	if cmcRow != nil {
		if cmc, err := strconv.Atoi(getRowValue(cmcRow).FirstChild.Data); err == nil {
			card.ConvertedManaCost = cmc
		}
	}
	if typeRow != nil {
		card.Type = strings.TrimSpace(getRowValue(typeRow).FirstChild.Data)
	}
	if textRow != nil {
		card.Text = strings.TrimSpace(getRowValue(textRow).FirstChild.NextSibling.FirstChild.Data)
	}

	return card, nil
}

func nodeSearch(root *html.Node, f func(*html.Node) bool, stopAtOne bool) (nodes []*html.Node) {
	queue := list.New()
	queue.PushBack(root)
	for queue.Len() != 0 {
		node := queue.Remove(queue.Front()).(*html.Node)
		if f(node) {
			nodes = append(nodes, node)
			if stopAtOne {
				return
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			queue.PushBack(child)
		}
	}
	return
}

func findNode(root *html.Node, f func(*html.Node) bool) *html.Node {
	nodes := nodeSearch(root, f, true)
	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

func findAllNodes(root *html.Node, f func(*html.Node) bool) (nodes []*html.Node) {
	return nodeSearch(root, f, false)
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

// Ripped from the standard library.
func resolvePath(base, ref string) string {
	var full string
	if ref == "" {
		full = base
	} else if ref[0] != '/' {
		i := strings.LastIndex(base, "/")
		full = base[:i+1] + ref
	} else {
		full = ref
	}
	if full == "" {
		return ""
	}
	var dst []string
	src := strings.Split(full, "/")
	for _, elem := range src {
		switch elem {
		case ".":
			// drop
		case "..":
			if len(dst) > 0 {
				dst = dst[:len(dst)-1]
			}
		default:
			dst = append(dst, elem)
		}
	}
	if last := src[len(src)-1]; last == "." || last == ".." {
		// Add final slash to the joined path.
		dst = append(dst, "")
	}
	return "/" + strings.TrimLeft(strings.Join(dst, "/"), "/")
}
