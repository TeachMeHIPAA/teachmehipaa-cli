package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/config"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/events"
)

const (
	eventsPageLimit = "100"
	// maxIdleInterval caps the backoff while the event log is quiet.
	maxIdleInterval = 10 * time.Second
)

// addEventCommands hangs the hand-written Event commands on the generated
// groups, so `tmh events --help` lists them beside `list`.
func addEventCommands(root *cobra.Command, f *cmdutil.Factory) {
	for _, group := range root.Commands() {
		switch group.Name() {
		case "events":
			group.AddCommand(newEventsTailCommand(f), newEventsResendCommand(f))
		case "webhook-subscriptions":
			// `tmh webhooks trigger-test` is the Stripe-style spelling of
			// `tmh webhook-subscriptions test`.
			group.Aliases = append(group.Aliases, "webhooks")
			for _, command := range group.Commands() {
				if command.Annotations["method"] == "webhookSubscriptions.test" {
					command.Aliases = append(command.Aliases, "trigger-test")
				}
			}
		}
	}
}

type tailOptions struct {
	after       string
	types       []string
	forwardTo   string
	interval    time.Duration
	fromStart   bool
	printSecret bool
}

func newEventsTailCommand(f *cmdutil.Factory) *cobra.Command {
	opts := &tailOptions{}
	command := &cobra.Command{
		Use:   "tail",
		Short: "Watch the Organization's Events as they happen, optionally forwarding them to a local endpoint",
		Long: `Watch the Organization's Events as they happen, optionally forwarding them to a local endpoint.

tail polls the event log and resumes where it last stopped for this profile
and Organization. The first run starts with the next new Event, unless
--from-start or --after says otherwise.

With --forward-to, each Event is POSTed to your endpoint signed exactly like a
TeachMeHIPAA Delivery, but with this profile's local Signing secret, so your
SDK's constructEvent verifies it. Press Ctrl-C to stop.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTail(cmd.Context(), f, opts)
		},
	}
	flags := command.Flags()
	flags.StringVar(&opts.after, "after", "", "start after this Event id instead of where the last tail stopped")
	flags.StringSliceVar(&opts.types, "types", nil, "only show these Event types, comma-separated")
	flags.StringVar(&opts.forwardTo, "forward-to", "", "URL to POST each Event to, such as http://localhost:3000/webhooks")
	flags.DurationVar(&opts.interval, "interval", 2*time.Second, "how often to poll the event log")
	flags.BoolVar(&opts.fromStart, "from-start", false, "start from the oldest Event in the log (the last 30 days)")
	flags.BoolVar(&opts.printSecret, "print-secret", false, "print the local Signing secret and exit")
	return command
}

func newEventsResendCommand(f *cmdutil.Factory) *cobra.Command {
	var forwardTo string
	command := &cobra.Command{
		Use:   "resend <eventId>",
		Short: "Forward one Event from the log to a local endpoint again",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResend(cmd.Context(), f, args[0], forwardTo)
		},
	}
	command.Flags().StringVar(&forwardTo, "forward-to", "", "URL to POST the Event to, such as http://localhost:3000/webhooks")
	_ = command.MarkFlagRequired("forward-to")
	return command
}

func localSigningSecret(f *cmdutil.Factory, profile config.Profile) (string, error) {
	return f.Store().LocalSigningSecret(profile.Name, events.NewSigningSecret)
}

func runTail(ctx context.Context, f *cmdutil.Factory, opts *tailOptions) error {
	profile, err := f.Profile()
	if err != nil {
		return err
	}
	if opts.printSecret {
		secret, err := localSigningSecret(f, profile)
		if err != nil {
			return err
		}
		fmt.Fprintln(f.Stdout, secret)
		return nil
	}
	if opts.interval <= 0 {
		return &cmdutil.UsageError{Message: "--interval must be more than 0"}
	}
	if opts.after != "" && opts.fromStart {
		return &cmdutil.UsageError{Message: "use --after or --from-start, not both"}
	}
	var forwarder *events.Forwarder
	secret, err := localSigningSecret(f, profile)
	if err != nil {
		return err
	}
	if opts.forwardTo != "" {
		target, err := events.ParseTarget(opts.forwardTo)
		if err != nil {
			return &cmdutil.UsageError{Message: err.Error()}
		}
		forwarder = events.NewForwarder(target, secret, f.Clock, f.ForwardTransport)
	}
	client, profile, err := f.APIClient()
	if err != nil {
		return err
	}
	organizationID, err := client.OrganizationID(ctx)
	if err != nil {
		return err
	}
	t := &tail{
		log:       eventLog{client: client, types: opts.types},
		f:         f,
		forwarder: forwarder,
		types:     opts.types,
		state:     config.StateStore{Dir: f.ConfigDir},
		profile:   profile.Name,
		org:       organizationID,
	}
	return t.run(ctx, opts, secret)
}

// eventLog lists the Organization's Events, oldest first.
type eventLog struct {
	client *api.Client
	types  []string
}

type eventsPage struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor *string           `json:"nextCursor"`
}

func (l eventLog) page(ctx context.Context, after string) (eventsPage, error) {
	query := url.Values{"limit": {eventsPageLimit}}
	if after != "" {
		query.Set("after", after)
	}
	// The API filters by one Event type; several are filtered here.
	if len(l.types) == 1 {
		query.Set("type", l.types[0])
	}
	response, err := l.client.Do(ctx, api.Request{
		Method:             http.MethodGet,
		Path:               "/events",
		OrganizationScoped: true,
		Query:              query,
		RetrySafe:          true,
	})
	if err != nil {
		return eventsPage{}, err
	}
	var page eventsPage
	if err := json.Unmarshal(response.Body, &page); err != nil {
		return eventsPage{}, fmt.Errorf("reading the events page: %w", err)
	}
	return page, nil
}

type tail struct {
	log       eventLog
	f         *cmdutil.Factory
	forwarder *events.Forwarder
	types     []string
	state     config.StateStore
	profile   string
	org       string
	cursor    string
	saved     string
}

func (t *tail) run(ctx context.Context, opts *tailOptions, secret string) error {
	if err := t.start(ctx, opts); err != nil {
		if ctx.Err() != nil {
			return t.save()
		}
		return err
	}
	if t.forwarder != nil {
		fmt.Fprintf(t.f.Stderr, "Ready! Your local signing secret is %s (^C to quit)\n", secret)
	} else {
		fmt.Fprintf(t.f.Stderr, "Ready! Tailing Events; your local signing secret is %s (^C to quit)\n", secret)
	}
	wait := opts.interval
	for {
		if ctx.Err() != nil {
			return t.save()
		}
		page, err := t.log.page(ctx, t.cursor)
		if err != nil {
			if ctx.Err() != nil {
				return t.save()
			}
			return firstError(err, t.save())
		}
		for _, raw := range page.Items {
			event, err := events.Parse(raw)
			if err != nil {
				return firstError(err, t.save())
			}
			if !t.handle(ctx, event) {
				return t.save()
			}
			t.cursor = event.ID
		}
		if err := t.save(); err != nil {
			return err
		}
		if len(page.Items) > 0 {
			wait = opts.interval
			if page.NextCursor != nil {
				continue
			}
		}
		if t.f.Clock.Sleep(ctx, wait) != nil {
			return t.save()
		}
		if len(page.Items) == 0 {
			wait = min(wait*2, max(maxIdleInterval, opts.interval))
		}
	}
}

// start picks the first cursor: --after, else where the last tail stopped,
// else the newest Event now, so a first run shows only new Events.
func (t *tail) start(ctx context.Context, opts *tailOptions) error {
	switch {
	case opts.fromStart:
		return nil
	case opts.after != "":
		t.cursor = opts.after
		return nil
	}
	state, err := t.state.Load()
	if err != nil {
		return err
	}
	if t.cursor = state.EventCursor(t.profile, t.org); t.cursor != "" {
		t.saved = t.cursor
		return nil
	}
	for {
		page, err := t.log.page(ctx, t.cursor)
		if err != nil {
			return err
		}
		if len(page.Items) > 0 {
			event, err := events.Parse(page.Items[len(page.Items)-1])
			if err != nil {
				return err
			}
			t.cursor = event.ID
		}
		if page.NextCursor == nil || len(page.Items) == 0 {
			return t.save()
		}
	}
}

// handle prints and forwards one Event. It reports false when Ctrl-C cut the
// forward short, so the Event isn't counted as seen and the next tail
// delivers it again.
func (t *tail) handle(ctx context.Context, event events.Event) bool {
	if len(t.types) > 1 && !slices.Contains(t.types, event.Type) {
		return true
	}
	line := fmt.Sprintf("%s  %s  %s", event.OccurredAt, event.Type, event.ID)
	if t.forwarder != nil {
		result := t.forwarder.Forward(ctx, event)
		if ctx.Err() != nil {
			return false
		}
		line += "  " + describeForward(t.forwarder, result)
	}
	fmt.Fprintln(t.f.Stdout, line)
	return true
}

func (t *tail) save() error {
	if t.cursor == "" || t.cursor == t.saved {
		return nil
	}
	err := t.state.Update(func(state *config.State) {
		state.SetEventCursor(t.profile, t.org, t.cursor)
	})
	if err == nil {
		t.saved = t.cursor
	}
	return err
}

func describeForward(forwarder *events.Forwarder, result events.Result) string {
	if result.Err != nil {
		return fmt.Sprintf("[error] POST %s: %v", forwarder.Target(), result.Err)
	}
	return fmt.Sprintf("[%d] POST %s (%dms)", result.Status, forwarder.Target(), result.Latency.Milliseconds())
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func runResend(ctx context.Context, f *cmdutil.Factory, eventID, forwardTo string) error {
	target, err := events.ParseTarget(forwardTo)
	if err != nil {
		return &cmdutil.UsageError{Message: err.Error()}
	}
	profile, err := f.Profile()
	if err != nil {
		return err
	}
	secret, err := localSigningSecret(f, profile)
	if err != nil {
		return err
	}
	client, _, err := f.APIClient()
	if err != nil {
		return err
	}
	event, err := findEvent(ctx, eventLog{client: client}, eventID)
	if err != nil {
		return err
	}
	forwarder := events.NewForwarder(target, secret, f.Clock, f.ForwardTransport)
	result := forwarder.Forward(ctx, event)
	fmt.Fprintf(f.Stdout, "%s  %s  %s  %s\n", event.OccurredAt, event.Type, event.ID, describeForward(forwarder, result))
	if !result.OK() {
		return fmt.Errorf("%s didn't accept Event %s", forwarder.Target(), event.ID)
	}
	return nil
}

// findEvent pages the log from its start: there is no endpoint for one
// Event.
func findEvent(ctx context.Context, log eventLog, eventID string) (events.Event, error) {
	after := ""
	for {
		page, err := log.page(ctx, after)
		if err != nil {
			return events.Event{}, err
		}
		for _, raw := range page.Items {
			event, err := events.Parse(raw)
			if err != nil {
				return events.Event{}, err
			}
			if event.ID == eventID {
				return event, nil
			}
			after = event.ID
		}
		if page.NextCursor == nil || len(page.Items) == 0 {
			return events.Event{}, &cmdutil.NotFoundError{
				Message: fmt.Sprintf("no Event %s in the event log, which keeps the last 30 days", eventID),
			}
		}
	}
}
