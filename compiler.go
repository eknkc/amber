package amber

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"github.com/eknkc/amber/parser"
	"go/ast"
	gp "go/parser"
	gt "go/token"
	"html/template"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var builtinFunctions = [...]string{
	"len",
	"print",
	"printf",
	"println",
	"urlquery",
	"js",
	"json",
	"index",
	"html",
	"unescaped",
}

// Compiler is the main interface of Amber Template Engine.
// In order to use an Amber template, it is required to create a Compiler and
// compile an Amber source to native Go template.
//	compiler := amber.New()
// 	// Parse the input file
//	err := compiler.ParseFile("./input.amber")
//	if err == nil {
//		// Compile input file to Go template
//		tpl, err := compiler.Compile()
//		if err == nil {
//			// Check built in html/template documentation for further details
//			tpl.Execute(os.Stdout, somedata)
//		}
//	}
type Compiler struct {
	// Compiler options
	Options
	filename     string
	node         parser.Node
	indentLevel  int
	newline      bool
	buffer       *bytes.Buffer
	tempvarIndex int
}

// Create and initialize a new Compiler
func New() *Compiler {
	compiler := new(Compiler)
	compiler.filename = ""
	compiler.tempvarIndex = 0
	compiler.PrettyPrint = true
	compiler.Options = DefaultOptions

	return compiler
}

type Options struct {
	// Setting if pretty printing is enabled.
	// Pretty printing ensures that the output html is properly indented and in human readable form.
	// If disabled, produced HTML is compact. This might be more suitable in production environments.
	// Default: true
	PrettyPrint bool
	// Setting if line number emitting is enabled
	// In this form, Amber emits line number comments in the output template. It is usable in debugging environments.
	// Default: false
	LineNumbers bool
}

var DefaultOptions = Options{true, false}

// Parses and compiles the supplied amber template string. Returns corresponding Go Template (html/templates) instance.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func Compile(input string, options Options) (*template.Template, error) {
	comp := New()
	comp.Options = options

	err := comp.Parse(input)
	if err != nil {
		return nil, err
	}

	return comp.Compile()
}

// Parses and compiles the contents of supplied filename. Returns corresponding Go Template (html/templates) instance.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func CompileFile(filename string, options Options) (*template.Template, error) {
	comp := New()
	comp.Options = options

	err := comp.ParseFile(filename)
	if err != nil {
		return nil, err
	}

	return comp.Compile()
}

// Parse given raw amber template string.
func (c *Compiler) Parse(input string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	parser, err := parser.StringParser(input)

	if err != nil {
		return
	}

	c.node = parser.Parse()
	return
}

// Parse the amber template file in given path
func (c *Compiler) ParseFile(filename string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	parser, err := parser.FileParser(filename)

	if err != nil {
		return
	}

	c.node = parser.Parse()
	c.filename = filename
	return
}

// Compile amber and create a Go Template (html/templates) instance.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func (c *Compiler) Compile() (*template.Template, error) {
	return c.CompileWithName(filepath.Base(c.filename))
}

// Same as Compile but allows to specify a name for the template
func (c *Compiler) CompileWithName(name string) (*template.Template, error) {
	return c.CompileWithTemplate(template.New(name))
}

// Same as Compile but allows to specify a template
func (c *Compiler) CompileWithTemplate(t *template.Template) (*template.Template, error) {
	data, err := c.CompileString()

	if err != nil {
		return nil, err
	}

	tpl, err := t.Funcs(funcMap).Parse(data)

	if err != nil {
		return nil, err
	}

	return tpl, nil
}

// Compile amber and write the Go Template source into given io.Writer instance
// You would not be using this unless debugging / checking the output. Please use Compile
// method to obtain a template instance directly.
func (c *Compiler) CompileWriter(out io.Writer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	c.buffer = new(bytes.Buffer)
	c.visit(c.node)

	if c.buffer.Len() > 0 {
		c.write("\n")
	}

	_, err = c.buffer.WriteTo(out)
	return
}

// Compile template and return the Go Template source
// You would not be using this unless debugging / checking the output. Please use Compile
// method to obtain a template instance directly.
func (c *Compiler) CompileString() (string, error) {
	var buf bytes.Buffer

	if err := c.CompileWriter(&buf); err != nil {
		return "", err
	}

	result := buf.String()

	return result, nil
}

func (c *Compiler) visit(node parser.Node) {
	defer func() {
		if r := recover(); r != nil {
			if rs, ok := r.(string); ok && rs[:len("Amber Error")] == "Amber Error" {
				panic(r)
			}

			pos := node.Pos()

			if len(pos.Filename) > 0 {
				panic(fmt.Sprintf("Amber Error in <%s>: %v - Line: %d, Column: %d, Length: %d", pos.Filename, r, pos.LineNum, pos.ColNum, pos.TokenLength))
			} else {
				panic(fmt.Sprintf("Amber Error: %v - Line: %d, Column: %d, Length: %d", r, pos.LineNum, pos.ColNum, pos.TokenLength))
			}
		}
	}()

	switch node.(type) {
	case *parser.Block:
		c.visitBlock(node.(*parser.Block))
	case *parser.Doctype:
		c.visitDoctype(node.(*parser.Doctype))
	case *parser.Comment:
		c.visitComment(node.(*parser.Comment))
	case *parser.Tag:
		c.visitTag(node.(*parser.Tag))
	case *parser.Text:
		c.visitText(node.(*parser.Text))
	case *parser.Condition:
		c.visitCondition(node.(*parser.Condition))
	case *parser.Each:
		c.visitEach(node.(*parser.Each))
	case *parser.Assignment:
		c.visitAssignment(node.(*parser.Assignment))
	}
}

func (c *Compiler) write(value string) {
	c.buffer.WriteString(value)
}

func (c *Compiler) indent(offset int, newline bool) {
	if !c.PrettyPrint {
		return
	}

	if newline && c.buffer.Len() > 0 {
		c.write("\n")
	}

	for i := 0; i < c.indentLevel+offset; i++ {
		c.write("\t")
	}
}

func (c *Compiler) tempvar() string {
	c.tempvarIndex++
	return "$__amber_" + strconv.Itoa(c.tempvarIndex)
}

func (c *Compiler) escape(input string) string {
	return strings.Replace(strings.Replace(input, `\`, `\\`, -1), `"`, `\"`, -1)
}

func (c *Compiler) visitBlock(block *parser.Block) {
	for _, node := range block.Children {
		if _, ok := node.(*parser.Text); !block.CanInline() && ok {
			c.indent(0, true)
		}

		c.visit(node)
	}
}

func (c *Compiler) visitDoctype(doctype *parser.Doctype) {
	c.write(doctype.String())
}

func (c *Compiler) visitComment(comment *parser.Comment) {
	if comment.Silent {
		return
	}

	c.indent(0, false)

	if comment.Block == nil {
		c.write(`{{unescaped "<!-- ` + c.escape(comment.Value) + ` -->"}}`)
	} else {
		c.write(`<!-- ` + comment.Value)
		c.visitBlock(comment.Block)
		c.write(` -->`)
	}
}

func (c *Compiler) visitCondition(condition *parser.Condition) {
	c.write(`{{if ` + c.visitRawInterpolation(condition.Expression) + `}}`)
	c.visitBlock(condition.Positive)
	if condition.Negative != nil {
		c.write(`{{else}}`)
		c.visitBlock(condition.Negative)
	}
	c.write(`{{end}}`)
}

func (c *Compiler) visitEach(each *parser.Each) {
	if each.Block == nil {
		return
	}

	if len(each.Y) == 0 {
		c.write(`{{range ` + each.X + ` := ` + c.visitRawInterpolation(each.Expression) + `}}`)
	} else {
		c.write(`{{range ` + each.X + `, ` + each.Y + ` := ` + c.visitRawInterpolation(each.Expression) + `}}`)
	}
	c.visitBlock(each.Block)
	c.write(`{{end}}`)
}

func (c *Compiler) visitAssignment(assgn *parser.Assignment) {
	c.write(`{{` + assgn.X + ` := ` + c.visitRawInterpolation(assgn.Expression) + `}}`)
}

func (c *Compiler) visitTag(tag *parser.Tag) {
	type attrib struct {
		name      string
		value     string
		condition string
	}

	attribs := make(map[string]*attrib)

	for _, item := range tag.Attributes {
		attr := new(attrib)
		attr.name = item.Name

		if !item.IsRaw {
			attr.value = c.visitInterpolation(item.Value)
		} else if item.Value == "" {
			attr.value = ""
		} else {
			attr.value = `{{"` + item.Value + `"}}`
		}

		if len(item.Condition) != 0 {
			attr.condition = c.visitRawInterpolation(item.Condition)
		}

		if attr.name == "class" && attribs["class"] != nil {
			prevclass := attribs["class"]
			attr.value = ` ` + attr.value

			if len(attr.condition) > 0 {
				attr.value = `{{if ` + attr.condition + `}}` + attr.value + `{{end}}`
				attr.condition = ""
			}

			if len(prevclass.condition) > 0 {
				prevclass.value = `{{if ` + prevclass.condition + `}}` + prevclass.value + `{{end}}`
				prevclass.condition = ""
			}

			prevclass.value = prevclass.value + attr.value
		} else {
			attribs[item.Name] = attr
		}
	}

	c.indent(0, true)
	c.write("<" + tag.Name)

	for name, value := range attribs {
		if len(value.condition) > 0 {
			c.write(`{{if ` + value.condition + `}}`)
		}

		if value.value == "" {
			c.write(` ` + name)
		} else {
			c.write(` ` + name + `="` + value.value + `"`)
		}

		if len(value.condition) > 0 {
			c.write(`{{end}}`)
		}
	}

	if tag.IsSelfClosing() {
		c.write(` />`)
	} else {
		c.write(`>`)

		if tag.Block != nil {
			if !tag.Block.CanInline() {
				c.indentLevel++
			}

			c.visitBlock(tag.Block)

			if !tag.Block.CanInline() {
				c.indentLevel--
				c.indent(0, true)
			}
		}

		c.write(`</` + tag.Name + `>`)
	}
}

var textInterpolateRegexp = regexp.MustCompile(`#\{(.*?)\}`)
var textEscapeRegexp = regexp.MustCompile(`\{\{(.*?)\}\}`)

func (c *Compiler) visitText(txt *parser.Text) {
	value := textEscapeRegexp.ReplaceAllStringFunc(txt.Value, func(value string) string {
		return `{{"{{"}}` + value[2:len(value)-2] + `{{"}}"}}`
	})

	value = textInterpolateRegexp.ReplaceAllStringFunc(value, func(value string) string {
		return c.visitInterpolation(value[2 : len(value)-1])
	})

	lines := strings.Split(value, "\n")
	for i := 0; i < len(lines); i++ {
		c.write(lines[i])

		if i < len(lines)-1 {
			c.write("\n")
			c.indent(0, false)
		}
	}
}

func (c *Compiler) visitInterpolation(value string) string {
	return `{{` + c.visitRawInterpolation(value) + `}}`
}

func (c *Compiler) visitRawInterpolation(value string) string {
	value = strings.Replace(value, "$", "__DOLLAR__", -1)
	expr, err := gp.ParseExpr(value)
	if err != nil {
		panic("Unable to parse expression.")
	}
	value = strings.Replace(c.visitExpression(expr), "__DOLLAR__", "$", -1)
	return value
}

func (c *Compiler) visitExpression(outerexpr ast.Expr) string {
	stack := list.New()

	pop := func() string {
		if stack.Front() == nil {
			return ""
		}

		val := stack.Front().Value.(string)
		stack.Remove(stack.Front())
		return val
	}

	var exec func(ast.Expr)

	exec = func(expr ast.Expr) {
		switch expr.(type) {
		case *ast.BinaryExpr:
			{
				be := expr.(*ast.BinaryExpr)

				exec(be.Y)
				exec(be.X)

				negate := false
				name := c.tempvar()
				c.write(`{{` + name + ` := `)

				switch be.Op {
				case gt.ADD:
					c.write("__amber_add ")
				case gt.SUB:
					c.write("__amber_sub ")
				case gt.MUL:
					c.write("__amber_mul ")
				case gt.QUO:
					c.write("__amber_quo ")
				case gt.REM:
					c.write("__amber_rem ")
				case gt.LAND:
					c.write("and ")
				case gt.LOR:
					c.write("or ")
				case gt.EQL:
					c.write("__amber_eql ")
				case gt.NEQ:
					c.write("__amber_eql ")
					negate = true
				case gt.LSS:
					c.write("__amber_lss ")
				case gt.GTR:
					c.write("__amber_gtr ")
				case gt.LEQ:
					c.write("__amber_gtr ")
					negate = true
				case gt.GEQ:
					c.write("__amber_lss ")
					negate = true
				default:
					panic("Unexpected operator!")
				}

				c.write(pop() + ` ` + pop() + `}}`)

				if !negate {
					stack.PushFront(name)
				} else {
					negname := c.tempvar()
					c.write(`{{` + negname + ` := not ` + name + `}}`)
					stack.PushFront(negname)
				}
			}
		case *ast.UnaryExpr:
			{
				ue := expr.(*ast.UnaryExpr)

				exec(ue.X)

				name := c.tempvar()
				c.write(`{{` + name + ` := `)

				switch ue.Op {
				case gt.SUB:
					c.write("__amber_minus ")
				case gt.ADD:
					c.write("__amber_plus ")
				case gt.NOT:
					c.write("not ")
				default:
					panic("Unexpected operator!")
				}

				c.write(pop() + `}}`)
				stack.PushFront(name)
			}
		case *ast.ParenExpr:
			exec(expr.(*ast.ParenExpr).X)
		case *ast.BasicLit:
			stack.PushFront(expr.(*ast.BasicLit).Value)
		case *ast.Ident:
			name := expr.(*ast.Ident).Name
			if len(name) >= len("__DOLLAR__") && name[:len("__DOLLAR__")] == "__DOLLAR__" {
				if name == "__DOLLAR__" {
					stack.PushFront(`.`)
				} else {
					stack.PushFront(`$` + expr.(*ast.Ident).Name[len("__DOLLAR__"):])
				}
			} else {
				stack.PushFront(`.` + expr.(*ast.Ident).Name)
			}
		case *ast.SelectorExpr:
			se := expr.(*ast.SelectorExpr)
			exec(se.X)
			x := pop()

			if x == "." {
				x = ""
			}

			name := c.tempvar()
			c.write(`{{` + name + ` := ` + x + `.` + se.Sel.Name + `}}`)
			stack.PushFront(name)
		case *ast.CallExpr:
			ce := expr.(*ast.CallExpr)

			for i := len(ce.Args) - 1; i >= 0; i-- {
				exec(ce.Args[i])
			}

			name := c.tempvar()
			builtin := false

			if ident, ok := ce.Fun.(*ast.Ident); ok {
				for _, fname := range builtinFunctions {
					if fname == ident.Name {
						builtin = true
						break
					}
				}
			}

			if builtin {
				stack.PushFront(ce.Fun.(*ast.Ident).Name)
				c.write(`{{` + name + ` := ` + pop())
			} else {
				exec(ce.Fun)
				c.write(`{{` + name + ` := call ` + pop())
			}

			for i := 0; i < len(ce.Args); i++ {
				c.write(` `)
				c.write(pop())
			}

			c.write(`}}`)

			stack.PushFront(name)
		default:
			panic("Unable to parse expression. Unsupported: " + reflect.TypeOf(expr).String())
		}
	}

	exec(outerexpr)
	return pop()
}
