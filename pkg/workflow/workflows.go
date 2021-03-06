package workflow

import (
	"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetWorkflow returns the workflow for the given name. If the name is blank it defaults to `DefaultWorkflowName`.
// If the workflow does not exist yet then its defaulted from the auto promotion workflows in order.
func GetWorkflow(name string, jxClient versioned.Interface, ns string) (*v1.Workflow, error) {
	if name == "" {
		name = DefaultWorkflowName
	}
	workflow, err := jxClient.JenkinsV1().Workflows(ns).Get(name, metav1.GetOptions{})
	if err == nil || name != DefaultWorkflowName {
		return workflow, err
	}
	m, names, err := kube.GetOrderedEnvironments(jxClient, ns)
	if err != nil {
		return nil, err
	}

	// lets create a default workflow
	workflow = &v1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultWorkflowName,
			Namespace: ns,
		},
		Spec: v1.WorkflowSpec{},
	}
	spec := &workflow.Spec
	for _, name := range names {
		env := m[name]
		if env != nil && env.Spec.PromotionStrategy == v1.PromotionStrategyTypeAutomatic && env.Spec.Kind == v1.EnvironmentKindTypePermanent {
			step := CreateWorkflowPromoteStep(name, false)
			spec.Steps = append(spec.Steps, step)
		}
	}
	return workflow, nil
}

// CreateWorkflow creates a default Workflow instance
func CreateWorkflow(ns string, name string, steps ...v1.WorkflowStep) *v1.Workflow {
	return &v1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kube.ToValidName(name),
			Namespace: ns,
		},
		Spec: v1.WorkflowSpec{
			Steps: steps,
		},
	}
}

// CreateWorkflowPromoteStep creates a default Workflow promote step
func CreateWorkflowPromoteStep(envName string, parallel bool) v1.WorkflowStep {
	return v1.WorkflowStep{
		Kind: v1.WorkflowStepKindTypePromote,
		Name: "step-promote-" + envName,
		Promote: &v1.PromoteWorkflowStep{
			Environment: envName,
			Parallel:    parallel,
		},
	}
}
