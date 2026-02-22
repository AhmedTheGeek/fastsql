package components

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/AhmedTheGeek/fastsql/internal/ai"
	"github.com/AhmedTheGeek/fastsql/internal/schema"
)

// AIPanel provides an interface for AI-powered SQL generation.
type AIPanel struct {
	*tview.Flex

	// UI components
	promptInput    *tview.TextArea
	outputView     *tview.TextView
	providerSelect *tview.DropDown
	statusText     *tview.TextView
	buttonsRow     *tview.Flex

	// State
	service       *ai.Service
	currentSchema *schema.Schema
	dialect       string
	generatedSQL  string
	isGenerating  bool
	mu            sync.Mutex

	// Callbacks
	onRun    func(sql string)
	onEdit   func(sql string)
	onClose  func()
	onCopy   func(sql string)
}

// NewAIPanel creates a new AI assistant panel.
func NewAIPanel(service *ai.Service) *AIPanel {
	panel := &AIPanel{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		service: service,
	}

	panel.setupUI()
	panel.setupKeybindings()

	return panel
}

func (p *AIPanel) setupUI() {
	// Header with provider selector
	header := tview.NewFlex().SetDirection(tview.FlexColumn)
	
	titleText := tview.NewTextView().
		SetText(" 🤖 AI Assistant ").
		SetTextColor(tcell.ColorWhite).
		SetDynamicColors(true)
	
	p.providerSelect = tview.NewDropDown().
		SetLabel("Provider: ").
		SetFieldWidth(20)
	
	// Populate providers
	if p.service != nil {
		providers := p.service.ListProviders()
		options := make([]string, len(providers))
		for i, id := range providers {
			options[i] = id
		}
		p.providerSelect.SetOptions(options, func(text string, index int) {
			p.service.SetActiveProvider(text)
		})
		// Set current provider
		currentID := p.service.GetActiveProviderID()
		for i, id := range providers {
			if id == currentID {
				p.providerSelect.SetCurrentOption(i)
				break
			}
		}
	}

	header.AddItem(titleText, 16, 0, false)
	header.AddItem(p.providerSelect, 0, 1, false)

	// Prompt input area
	promptLabel := tview.NewTextView().
		SetText(" Enter your query in natural language: ").
		SetTextColor(tcell.ColorYellow)

	p.promptInput = tview.NewTextArea().
		SetPlaceholder("e.g., Show me all users who signed up in the last 30 days...").
		SetWordWrap(true)
	p.promptInput.SetBorder(true).
		SetTitle(" Prompt ").
		SetBorderColor(tcell.ColorBlue)

	// Output view for generated SQL
	p.outputView = tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true).
		SetScrollable(true)
	p.outputView.SetBorder(true).
		SetTitle(" Generated SQL ").
		SetBorderColor(tcell.ColorGreen)

	// Status text
	p.statusText = tview.NewTextView().
		SetText(" Press Enter to generate | Tab to accept | Esc to close ").
		SetTextColor(tcell.ColorGray)

	// Action buttons row
	p.buttonsRow = tview.NewFlex().SetDirection(tview.FlexColumn)
	
	runBtn := tview.NewButton("Run").SetSelectedFunc(func() {
		if p.onRun != nil && p.generatedSQL != "" {
			p.onRun(p.generatedSQL)
		}
	})
	
	editBtn := tview.NewButton("Edit").SetSelectedFunc(func() {
		if p.onEdit != nil && p.generatedSQL != "" {
			p.onEdit(p.generatedSQL)
		}
	})
	
	copyBtn := tview.NewButton("Copy").SetSelectedFunc(func() {
		if p.onCopy != nil && p.generatedSQL != "" {
			p.onCopy(p.generatedSQL)
		}
	})
	
	explainBtn := tview.NewButton("Explain").SetSelectedFunc(func() {
		p.explainQuery()
	})

	p.buttonsRow.AddItem(runBtn, 8, 0, false)
	p.buttonsRow.AddItem(tview.NewBox(), 1, 0, false)
	p.buttonsRow.AddItem(editBtn, 8, 0, false)
	p.buttonsRow.AddItem(tview.NewBox(), 1, 0, false)
	p.buttonsRow.AddItem(copyBtn, 8, 0, false)
	p.buttonsRow.AddItem(tview.NewBox(), 1, 0, false)
	p.buttonsRow.AddItem(explainBtn, 10, 0, false)
	p.buttonsRow.AddItem(tview.NewBox(), 0, 1, false)

	// Layout
	p.AddItem(header, 1, 0, false)
	p.AddItem(promptLabel, 1, 0, false)
	p.AddItem(p.promptInput, 5, 0, true)
	p.AddItem(p.outputView, 0, 1, false)
	p.AddItem(p.buttonsRow, 1, 0, false)
	p.AddItem(p.statusText, 1, 0, false)

	p.SetBorder(true).
		SetTitle(" AI SQL Generator ").
		SetBorderColor(tcell.ColorTeal)
}

func (p *AIPanel) setupKeybindings() {
	p.promptInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			if event.Modifiers()&tcell.ModCtrl != 0 {
				// Ctrl+Enter: Run immediately
				if p.onRun != nil && p.generatedSQL != "" {
					p.onRun(p.generatedSQL)
				}
				return nil
			}
			// Enter: Generate SQL
			p.generateSQL()
			return nil

		case tcell.KeyTab:
			// Tab: Accept into editor
			if p.onEdit != nil && p.generatedSQL != "" {
				p.onEdit(p.generatedSQL)
			}
			return nil

		case tcell.KeyEsc:
			// Escape: Close panel
			if p.onClose != nil {
				p.onClose()
			}
			return nil
		}
		return event
	})
}

// SetSchema sets the database schema for context.
func (p *AIPanel) SetSchema(sch *schema.Schema, dialect string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentSchema = sch
	p.dialect = dialect
}

// SetCallbacks sets the callback functions.
func (p *AIPanel) SetCallbacks(onRun, onEdit, onCopy func(string), onClose func()) {
	p.onRun = onRun
	p.onEdit = onEdit
	p.onCopy = onCopy
	p.onClose = onClose
}

// GetPromptInput returns the prompt input component for focus management.
func (p *AIPanel) GetPromptInput() tview.Primitive {
	return p.promptInput
}

// Clear resets the panel.
func (p *AIPanel) Clear() {
	p.promptInput.SetText("", true)
	p.outputView.Clear()
	p.generatedSQL = ""
	p.updateStatus(" Press Enter to generate | Tab to accept | Esc to close ")
}

func (p *AIPanel) generateSQL() {
	p.mu.Lock()
	if p.isGenerating {
		p.mu.Unlock()
		return
	}
	prompt := p.promptInput.GetText()
	if strings.TrimSpace(prompt) == "" {
		p.mu.Unlock()
		return
	}
	p.isGenerating = true
	p.mu.Unlock()

	p.updateStatus(" ⏳ Generating... ")
	p.outputView.Clear()

	go func() {
		defer func() {
			p.mu.Lock()
			p.isGenerating = false
			p.mu.Unlock()
		}()

		if p.service == nil {
			p.showError("AI service not configured")
			return
		}

		req := &ai.GenerateRequest{
			Schema:  p.currentSchema,
			Prompt:  prompt,
			Dialect: p.dialect,
		}

		// Try streaming first
		ch, err := p.service.StreamSQL(context.Background(), req)
		if err != nil {
			p.showError(fmt.Sprintf("Failed to generate: %v", err))
			return
		}

		var fullSQL strings.Builder
		for chunk := range ch {
			if chunk.Error != nil {
				p.showError(fmt.Sprintf("Error: %v", chunk.Error))
				return
			}
			if chunk.Text != "" {
				fullSQL.WriteString(chunk.Text)
				// Update UI with streaming content
				p.updateOutput(fullSQL.String())
			}
			if chunk.Done {
				break
			}
		}

		p.mu.Lock()
		p.generatedSQL = strings.TrimSpace(fullSQL.String())
		p.mu.Unlock()

		p.updateStatus(fmt.Sprintf(" ✓ Generated | Press Tab to accept, Ctrl+Enter to run "))
	}()
}

func (p *AIPanel) explainQuery() {
	p.mu.Lock()
	if p.isGenerating || p.generatedSQL == "" {
		p.mu.Unlock()
		return
	}
	p.isGenerating = true
	p.mu.Unlock()

	p.updateStatus(" ⏳ Explaining... ")

	go func() {
		defer func() {
			p.mu.Lock()
			p.isGenerating = false
			p.mu.Unlock()
		}()

		if p.service == nil {
			p.showError("AI service not configured")
			return
		}

		req := &ai.GenerateRequest{
			Prompt: fmt.Sprintf("Explain this SQL query in simple terms:\n\n%s", p.generatedSQL),
		}

		resp, err := p.service.GenerateSQL(context.Background(), req)
		if err != nil {
			p.showError(fmt.Sprintf("Failed to explain: %v", err))
			return
		}

		// Show explanation below the SQL
		explanation := fmt.Sprintf("%s\n\n[yellow]Explanation:[white]\n%s", 
			p.generatedSQL, resp.SQL)
		p.updateOutput(explanation)
		p.updateStatus(" ✓ Explanation complete ")
	}()
}

func (p *AIPanel) updateOutput(text string) {
	// Thread-safe UI update
	p.outputView.SetText(text)
}

func (p *AIPanel) updateStatus(text string) {
	p.statusText.SetText(text)
}

func (p *AIPanel) showError(msg string) {
	p.outputView.SetText(fmt.Sprintf("[red]Error: %s[white]", msg))
	p.updateStatus(" ✗ Generation failed ")
}

// Focus sets focus to the prompt input.
func (p *AIPanel) Focus(delegate func(p tview.Primitive)) {
	delegate(p.promptInput)
}
