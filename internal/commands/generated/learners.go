package generated

import (
	"net/http"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

// Register adds every resource command to root. This hand-written version
// carries one command per request kind the HTTP client treats differently
// (a GET, a POST with a required Idempotency-Key, a POST the IR marks
// retrySafe, a DELETE) until the generator replaces the package.
func Register(root *cobra.Command, f *cmdutil.Factory) {
	learners := &cobra.Command{Use: "learners", Short: "Learners in your Organization"}
	learners.AddCommand(newLearnersRetrieve(f), newLearnersCreateLaunchLink(f))
	enrollments := &cobra.Command{Use: "enrollments", Short: "Learners' Enrollments in Courses"}
	enrollments.AddCommand(newEnrollmentsRemind(f))
	webhookSubscriptions := &cobra.Command{Use: "webhook-subscriptions", Short: "Where your Events are delivered"}
	webhookSubscriptions.AddCommand(newWebhookSubscriptionsDelete(f))
	root.AddCommand(learners, enrollments, webhookSubscriptions)
}

// run sends one request and prints the response body.
func run(cmd *cobra.Command, f *cmdutil.Factory, request api.Request) error {
	client, _, err := f.APIClient()
	if err != nil {
		return err
	}
	response, err := client.Do(cmd.Context(), request)
	if err != nil {
		return err
	}
	return f.PrintJSON(response.Body)
}

func newLearnersCreateLaunchLink(f *cmdutil.Factory) *cobra.Command {
	var courseID string
	command := &cobra.Command{
		Use:   "create-launch-link <learnerId>",
		Short: "Create a launch link",
		Long: "Creates a link that opens the course with the learner already signed in, for a \"Start training\" button in your product. " +
			"The link works once and expires after 5 minutes: create it when the learner clicks, then redirect them to it. " +
			"Never email, store or log it. Only for a learner who has accepted their invite.\n\nScopes: learners:write",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := url.Values{}
			if courseID != "" {
				query.Set("courseId", courseID)
			}
			return run(cmd, f, api.Request{
				Method:             http.MethodPost,
				Path:               "/learners/" + url.PathEscape(args[0]) + ":createLaunchLink",
				OrganizationScoped: true,
				Query:              query,
				RetrySafe:          true,
				IdempotencyKey:     f.IdempotencyKey(false),
			})
		},
	}
	command.Flags().StringVar(&courseID, "course-id", "", "the Course; defaults to the Organization's default Course")
	return command
}

func newEnrollmentsRemind(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "remind <enrollmentId>",
		Short: "Remind an enrollment",
		Long: "Emails the learner again: the invitation if they haven't joined, or a reminder linking to the training if they have." +
			"\n\nScopes: enrollments:write",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, f, api.Request{
				Method:             http.MethodPost,
				Path:               "/enrollments/" + url.PathEscape(args[0]) + ":remind",
				OrganizationScoped: true,
				RetrySafe:          true,
				IdempotencyKey:     f.IdempotencyKey(true),
			})
		},
	}
}

func newWebhookSubscriptionsDelete(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <subscriptionId>",
		Short: "Delete a webhook subscription",
		Long:  "Deletes a webhook subscription and its signing secret.\n\nScopes: webhooks:manage or org:admin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, f, api.Request{
				Method:             http.MethodDelete,
				Path:               "/webhook-subscriptions/" + url.PathEscape(args[0]),
				OrganizationScoped: true,
				IdempotencyKey:     f.IdempotencyKey(false),
			})
		},
	}
}

func newLearnersRetrieve(f *cmdutil.Factory) *cobra.Command {
	var courseID string
	command := &cobra.Command{
		Use:   "retrieve <learnerId>",
		Short: "Retrieve a Learner's status in one Course",
		Long:  "Retrieves a Learner's status in one Course.\n\nScopes: learners:read",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := url.Values{}
			if courseID != "" {
				query.Set("courseId", courseID)
			}
			return run(cmd, f, api.Request{
				Method:             http.MethodGet,
				Path:               "/learners/" + url.PathEscape(args[0]),
				OrganizationScoped: true,
				Query:              query,
			})
		},
	}
	command.Flags().StringVar(&courseID, "course-id", "", "the Course; defaults to the Organization's default Course")
	return command
}
