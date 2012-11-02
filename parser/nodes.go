package parser

var selfClosingTags = [...]string{
	"meta",
	"img",
	"link",
	"input",
	"source",
	"area",
	"base",
	"col",
	"br",
	"hr",
}

var doctypes = map[string]string{
	"5":            `<!DOCTYPE html>`,
	"default":      `<!DOCTYPE html>`,
	"xml":          `<?xml version="1.0" encoding="utf-8" ?>`,
	"transitional": `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">`,
	"strict":       `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">`,
	"frameset":     `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Frameset//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-frameset.dtd">`,
	"1.1":          `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">`,
	"basic":        `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML Basic 1.1//EN" "http://www.w3.org/TR/xhtml-basic/xhtml-basic11.dtd">`,
	"mobile":       `<!DOCTYPE html PUBLIC "-//WAPFORUM//DTD XHTML Mobile 1.2//EN" "http://www.openmobilealliance.org/tech/DTD/xhtml-mobile12.dtd">`,
}

type Node interface {
	Pos() SourcePosition
}

type SourcePosition struct {
	LineNum     int
	ColNum      int
	TokenLength int
	Filename    string
}

func (s *SourcePosition) Pos() SourcePosition {
	return *s
}

type Doctype struct {
	SourcePosition
	Value string
}

func newDoctype(value string) *Doctype {
	dt := new(Doctype)
	dt.Value = value
	return dt
}

func (d *Doctype) String() string {
	if defined := doctypes[d.Value]; len(defined) != 0 {
		return defined
	}

	return `<!DOCTYPE ` + d.Value + `>`
}

type Comment struct {
	SourcePosition
	Value  string
	Block  *Block
	Silent bool
}

func newComment(value string) *Comment {
	dt := new(Comment)
	dt.Value = value
	dt.Block = nil
	dt.Silent = false
	return dt
}

type Text struct {
	SourcePosition
	Value string
	Raw   bool
}

func newText(value string, raw bool) *Text {
	dt := new(Text)
	dt.Value = value
	dt.Raw = raw
	return dt
}

type Block struct {
	SourcePosition
	Children []Node
}

func newBlock() *Block {
	block := new(Block)
	block.Children = make([]Node, 0)
	return block
}

func (b *Block) push(node Node) {
	b.Children = append(b.Children, node)
}

func (b *Block) pushFront(node Node) {
	b.Children = append([]Node{node}, b.Children...)
}

func (b *Block) CanInline() bool {
	if len(b.Children) == 0 {
		return true
	}

	allText := true

	for _, child := range b.Children {
		if txt, ok := child.(*Text); !ok || txt.Raw {
			allText = false
			break
		}
	}

	return allText
}

const (
	NamedBlockDefault = iota
	NamedBlockAppend
	NamedBlockPrepend
)

type NamedBlock struct {
	Block
	Name     string
	Modifier int
}

func newNamedBlock(name string) *NamedBlock {
	bb := new(NamedBlock)
	bb.Name = name
	bb.Block.Children = make([]Node, 0)
	bb.Modifier = NamedBlockDefault
	return bb
}

type Attribute struct {
	SourcePosition
	Name      string
	Value     string
	IsRaw     bool
	Condition string
}

type Tag struct {
	SourcePosition
	Block          *Block
	Name           string
	IsInterpolated bool
	Attributes     []Attribute
}

func newTag(name string) *Tag {
	tag := new(Tag)
	tag.Block = nil
	tag.Name = name
	tag.Attributes = make([]Attribute, 0)
	tag.IsInterpolated = false
	return tag

}

func (t *Tag) IsSelfClosing() bool {
	for _, tag := range selfClosingTags {
		if tag == t.Name {
			return true
		}
	}

	return false
}

func (t *Tag) IsRawText() bool {
	return t.Name == "style" || t.Name == "script"
}

type Condition struct {
	SourcePosition
	Positive   *Block
	Negative   *Block
	Expression string
}

func newCondition(exp string) *Condition {
	cond := new(Condition)
	cond.Expression = exp
	return cond
}

type Each struct {
	SourcePosition
	X          string
	Y          string
	Expression string
	Block      *Block
}

func newEach(exp string) *Each {
	each := new(Each)
	each.Expression = exp
	return each
}

type Assignment struct {
	SourcePosition
	X          string
	Expression string
}

func newAssignment(x, expression string) *Assignment {
	assgn := new(Assignment)
	assgn.X = x
	assgn.Expression = expression
	return assgn
}
