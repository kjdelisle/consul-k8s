package deletecompletedjob

import (
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
	batch "k8s.io/api/batch/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"math/rand"
	"testing"
	"time"
)

func TestRun_ArgValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		args   []string
		expErr string
	}{
		{
			[]string{},
			"Must have one arg: the job name to delete.",
		},
	}
	for _, c := range cases {
		t.Run(c.expErr, func(t *testing.T) {
			k8s := fake.NewSimpleClientset()
			ui := cli.NewMockUi()
			cmd := Command{
				UI:        ui,
				k8sClient: k8s,
			}
			cmd.init()
			responseCode := cmd.Run(c.args)
			require.Equal(t, 1, responseCode)
			require.Contains(t, ui.ErrorWriter.String(), c.expErr)
		})
	}
}

func TestRun_JobDoesNotExist(t *testing.T) {
	t.Parallel()
	ns := "default"
	jobName := "job"
	k8s := fake.NewSimpleClientset()
	ui := cli.NewMockUi()
	cmd := Command{
		UI:        ui,
		k8sClient: k8s,
	}
	cmd.init()

	responseCode := cmd.Run([]string{
		"-k8s-namespace", ns,
		jobName,
	})
	require.Equal(t, 0, responseCode, ui.ErrorWriter.String())
}

// Test when the job condition changes to either success or failed.
func TestRun_JobConditionChanges(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	cases := map[string]struct {
		EventualStatus batch.JobStatus
		ExpDelete      bool
	}{
		"job fails": {
			EventualStatus: batch.JobStatus{
				Active: 0,
				Failed: 1,
				Conditions: []batch.JobCondition{
					{
						Type:    batch.JobFailed,
						Status:  "True",
						Reason:  "BackoffLimitExceeded",
						Message: "Job has reached the specified backoff limit",
					},
				},
			},
			ExpDelete: false,
		},
		"job succeeds": {
			EventualStatus: batch.JobStatus{
				Succeeded: 1,
				Conditions: []batch.JobCondition{
					{
						Type:   batch.JobComplete,
						Status: "True",
					},
				},
			},
			ExpDelete: true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ns := "default"
			jobName := "job"
			k8s := fake.NewSimpleClientset()

			// Create the job that's not complete.
			_, err := k8s.BatchV1().Jobs(ns).Create(&batch.Job{
				ObjectMeta: meta.ObjectMeta{
					Name: jobName,
				},
				Status: batch.JobStatus{
					Active: 1,
				},
			})
			require.NoError(err)

			ui := cli.NewMockUi()
			cmd := Command{
				UI:        ui,
				k8sClient: k8s,
				// Set a low retry for tests.
				retryDuration: 20 * time.Millisecond,
			}
			cmd.init()

			// Start the command before the Pod exist.
			// Run in a goroutine so we can create the Pods asynchronously
			done := make(chan bool)
			var responseCode int
			go func() {
				responseCode = cmd.Run([]string{
					"-k8s-namespace", ns,
					jobName,
				})
				close(done)
			}()

			// Asynchronously update the job to be complete.
			go func() {
				// Update after a delay between 100 and 500ms.
				// It's randomized to ensure we're not relying on specific timing.
				delay := 100 + rand.Intn(400)
				time.Sleep(time.Duration(delay) * time.Millisecond)

				_, err := k8s.BatchV1().Jobs(ns).Update(&batch.Job{
					ObjectMeta: meta.ObjectMeta{
						Name: jobName,
					},
					Status: c.EventualStatus,
				})
				require.NoError(err)
			}()

			// Wait for the command to exit.
			select {
			case <-done:
				require.Equal(0, responseCode, ui.ErrorWriter.String())
			case <-time.After(2 * time.Second):
				require.FailNow("command did not exit after 2s")
			}

			// Check job deletion.
			_, err = k8s.BatchV1().Jobs(ns).Get(jobName, meta.GetOptions{})
			if c.ExpDelete {
				require.True(k8serrors.IsNotFound(err))
			} else {
				require.NoError(err)
			}
		})
	}
}