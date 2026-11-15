package generated

import (
	"net/http"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/api"
	"github.com/TeachMeHIPAA/teachmehipaa-cli/internal/cmdutil"
)

// Register adds every resource command to root. This hand-written version
// carries only `learners retrieve` until the generator replaces the package.
func Register(root *cobra.Command, f *cmdutil.Factory) {
	learners := &cobra.Command{Use: "learners", Short: "Learners in your Organization"}
	learners.AddCommand(newLearnersRetrieve(f))
	root.AddCommand(learners)
}

func newLearnersRetrieve(f *cmdutil.Factory) *cobra.Command {
	var courseID string
	command := &cobra.Command{
		Use:   "retrieve <learnerId>",
		Short: "Retrieve a Learner's status in one Course",
		Long:  "Retrieves a Learner's status in one Course.\n\nScopes: learners:read",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := f.APIClient()
			if err != nil {
				return err
			}
			query := url.Values{}
			if courseID != "" {
				query.Set("courseId", courseID)
			}
			response, err := client.Do(cmd.Context(), api.Request{
				Method:             http.MethodGet,
				Path:               "/learners/" + url.PathEscape(args[0]),
				OrganizationScoped: true,
				Query:              query,
			})
			if err != nil {
				return err
			}
			return f.PrintJSON(response.Body)
		},
	}
	command.Flags().StringVar(&courseID, "course-id", "", "the Course; defaults to the Organization's default Course")
	return command
}
