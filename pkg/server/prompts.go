// ABOUTME: MCP prompt templates for common GSuite workflows
// ABOUTME: Pre-defined prompts that help users accomplish common tasks

package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerPrompts registers all MCP prompts
func (s *Server) registerPrompts() {
	// Email triage prompt
	s.mcp.AddPrompt(
		mcp.NewPrompt(
			"email_triage",
			mcp.WithPromptDescription("Help triage and organize unread emails efficiently"),
			mcp.WithArgument("priority", mcp.ArgumentDescription("Priority level to focus on (urgent/normal/all)")),
		),
		s.handleEmailTriagePrompt,
	)

	// Meeting scheduler prompt
	s.mcp.AddPrompt(
		mcp.NewPrompt(
			"schedule_meeting",
			mcp.WithPromptDescription("Find available time slots and schedule a meeting"),
			mcp.WithArgument("duration", mcp.ArgumentDescription("Meeting duration in minutes (default: 30)")),
			mcp.WithArgument("attendees", mcp.ArgumentDescription("Comma-separated list of attendee emails")),
		),
		s.handleScheduleMeetingPrompt,
	)

	// Email composer prompt
	s.mcp.AddPrompt(
		mcp.NewPrompt(
			"compose_email",
			mcp.WithPromptDescription("Help compose a professional email"),
			mcp.WithArgument("context", mcp.ArgumentDescription("Context or purpose of the email"), mcp.RequiredArgument()),
			mcp.WithArgument("tone", mcp.ArgumentDescription("Desired tone (formal/casual/friendly)")),
		),
		s.handleComposeEmailPrompt,
	)

	// Contact finder prompt
	s.mcp.AddPrompt(
		mcp.NewPrompt(
			"find_contact",
			mcp.WithPromptDescription("Search for and retrieve contact information"),
			mcp.WithArgument("search_term", mcp.ArgumentDescription("Name, email, or organization to search for"), mcp.RequiredArgument()),
		),
		s.handleFindContactPrompt,
	)

	// Calendar summary prompt
	s.mcp.AddPrompt(
		mcp.NewPrompt(
			"calendar_summary",
			mcp.WithPromptDescription("Summarize calendar events for a time period"),
			mcp.WithArgument("period", mcp.ArgumentDescription("Time period (today/tomorrow/this_week/next_week)")),
		),
		s.handleCalendarSummaryPrompt,
	)

	// Follow-up reminder prompt
	s.mcp.AddPrompt(
		mcp.NewPrompt(
			"follow_up_reminder",
			mcp.WithPromptDescription("Set up follow-up reminders for important emails or meetings"),
			mcp.WithArgument("context", mcp.ArgumentDescription("What needs follow-up"), mcp.RequiredArgument()),
			mcp.WithArgument("when", mcp.ArgumentDescription("When to follow up (tomorrow/next_week/specific_date)")),
		),
		s.handleFollowUpReminderPrompt,
	)
}

// Prompt handlers

func (s *Server) handleEmailTriagePrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	priority := "all"
	if request.Params.Arguments != nil {
		if p, ok := request.Params.Arguments["priority"]; ok {
			priority = p
		}
	}

	query := "is:unread"
	if priority == "urgent" {
		query = "is:unread is:important"
	}

	promptText := fmt.Sprintf(`I'll help you triage your emails. Here's what I'll do:

1. **List your %s unread emails** using gmail_list_messages with query: "%s"
2. **Review each email** and categorize by:
   - Urgent action needed
   - Can wait / reply later
   - Informational only
   - Spam/unsubscribe candidate
3. **Suggest actions** for each category:
   - Quick replies for urgent items
   - Archiving for informational items
   - Unsubscribing from unwanted senders

Let me start by fetching your unread emails...`, priority, query)

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText)),
	}

	return mcp.NewGetPromptResult("Email triage workflow to help organize your inbox", messages), nil
}

func (s *Server) handleScheduleMeetingPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	duration := "30"
	attendees := ""

	if request.Params.Arguments != nil {
		if d, ok := request.Params.Arguments["duration"]; ok {
			duration = d
		}
		if a, ok := request.Params.Arguments["attendees"]; ok {
			attendees = a
		}
	}

	attendeeList := ""
	if attendees != "" {
		attendeeList = fmt.Sprintf("\n- Attendees: %s", attendees)
	}

	promptText := fmt.Sprintf(`I'll help you schedule a meeting. Here's my plan:

1. **Check your calendar** for the next 7 days using calendar_list_events
2. **Find available time slots** of %s minutes%s
3. **Suggest 3-5 best meeting times** considering:
   - No conflicts with existing events
   - Business hours (9 AM - 5 PM)
   - Buffer time between meetings (15 min)
   - Lunch time avoided (12-1 PM)
4. **Create the calendar event** once you choose a time

Let me start by checking your calendar availability...`, duration, attendeeList)

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText)),
	}

	return mcp.NewGetPromptResult("Meeting scheduling assistant to find optimal meeting times", messages), nil
}

func (s *Server) handleComposeEmailPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	emailContext := "a professional email"
	tone := "professional"

	if request.Params.Arguments != nil {
		if c, ok := request.Params.Arguments["context"]; ok && c != "" {
			emailContext = c
		}
		if t, ok := request.Params.Arguments["tone"]; ok && t != "" {
			tone = t
		}
	}

	promptText := fmt.Sprintf(`I'll help you compose %s with a %s tone.

**Email Structure:**
1. **Subject line**: Clear, specific, actionable
2. **Greeting**: Appropriate for the tone and relationship
3. **Opening**: Brief context or reference
4. **Body**: Main message in 2-3 short paragraphs
5. **Call to action**: Clear next steps
6. **Closing**: Professional sign-off

**Guidelines:**
- Keep it concise (under 200 words)
- Use bullet points for multiple items
- Proofread for clarity
- Consider recipient's time

Once I draft the email, I can:
- **Save as draft** using gmail_create_draft
- **Send immediately** using gmail_send_message
- **Iterate** based on your feedback

Tell me more about:
- Who is the recipient?
- What's the main message or request?
- Any specific details to include?`, emailContext, tone)

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText)),
	}

	return mcp.NewGetPromptResult("Email composition assistant for professional communication", messages), nil
}

func (s *Server) handleFindContactPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	searchTerm := ""

	if request.Params.Arguments != nil {
		if s, ok := request.Params.Arguments["search_term"]; ok {
			searchTerm = s
		}
	}

	if searchTerm == "" {
		return nil, fmt.Errorf("search_term argument is required")
	}

	promptText := fmt.Sprintf(`I'll help you find contact information for "%s".

**Search Strategy:**
1. **Search contacts** using people_search_contacts with query: "%s"
2. **Review results** and show:
   - Name and organization
   - Email addresses
   - Phone numbers
   - Last contact date (if available)
3. **Get detailed info** for the most relevant match using people_get_contact
4. **Suggest actions**:
   - Send an email
   - Schedule a meeting
   - Update contact information

Let me search your contacts now...`, searchTerm, searchTerm)

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText)),
	}

	return mcp.NewGetPromptResult("Contact search and information retrieval assistant", messages), nil
}

func (s *Server) handleCalendarSummaryPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	period := "today"

	if request.Params.Arguments != nil {
		if p, ok := request.Params.Arguments["period"]; ok {
			period = p
		}
	}

	var timeRange string
	var description string

	switch strings.ToLower(period) {
	case "today":
		timeRange = "the next 24 hours"
		description = "today's schedule"
	case "tomorrow":
		timeRange = "tomorrow (next 24-48 hours)"
		description = "tomorrow's schedule"
	case "this_week":
		timeRange = "this week (next 7 days)"
		description = "this week's schedule"
	case "next_week":
		timeRange = "next week (7-14 days from now)"
		description = "next week's schedule"
	default:
		timeRange = "the specified period"
		description = "schedule for the period"
	}

	promptText := fmt.Sprintf(`I'll provide a summary of %s.

**Calendar Analysis:**
1. **Fetch events** for %s using calendar_list_events
2. **Organize by day** and categorize:
   - Meetings with others
   - Focus/blocked time
   - All-day events
3. **Calculate statistics**:
   - Total meeting hours
   - Available/free time
   - Busiest day
4. **Highlight important events**:
   - Recurring meetings
   - Events with multiple attendees
   - Deadline-related events

**Summary Format:**
- Daily breakdown with times
- Meeting load assessment
- Suggestions for time management

Let me fetch and analyze your calendar...`, description, timeRange)

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText)),
	}

	return mcp.NewGetPromptResult("Calendar summary and analysis for time period", messages), nil
}

func (s *Server) handleFollowUpReminderPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	followUpContext := ""
	when := "tomorrow"

	if request.Params.Arguments != nil {
		if c, ok := request.Params.Arguments["context"]; ok {
			followUpContext = c
		}
		if w, ok := request.Params.Arguments["when"]; ok {
			when = w
		}
	}

	if followUpContext == "" {
		return nil, fmt.Errorf("context argument is required")
	}

	var reminderTime time.Time
	var timeDescription string

	now := time.Now()
	switch strings.ToLower(when) {
	case "tomorrow":
		reminderTime = now.Add(24 * time.Hour)
		timeDescription = "tomorrow at 9 AM"
	case "next_week":
		reminderTime = now.Add(7 * 24 * time.Hour)
		timeDescription = "next week at 9 AM"
	default:
		// Try to parse as date
		parsed, err := time.Parse("2006-01-02", when)
		if err != nil {
			reminderTime = now.Add(24 * time.Hour)
			timeDescription = "tomorrow at 9 AM"
		} else {
			reminderTime = parsed
			timeDescription = parsed.Format("January 2 at 9 AM")
		}
	}

	promptText := fmt.Sprintf(`I'll set up a follow-up reminder for: "%s"

**Follow-up Plan:**
1. **Create calendar reminder** for %s using calendar_create_event
   - Title: "Follow up: %s"
   - Description: Full context and action items
   - Reminder: 15 minutes before
2. **Send email to yourself** as backup using gmail_send_message
   - Subject: "Follow-up needed: %s"
   - Body: Context and specific action items
3. **Track in tasks** (optional):
   - Can integrate with Google Tasks if needed

**Reminder Details:**
- When: %s
- Context: %s
- Action: Review and take necessary follow-up steps

Let me create this follow-up reminder for you...`, followUpContext, timeDescription, followUpContext, followUpContext, reminderTime.Format(time.RFC3339), followUpContext)

	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText)),
	}

	return mcp.NewGetPromptResult("Follow-up reminder creation assistant", messages), nil
}
