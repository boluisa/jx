package kube

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/gits"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureGitServiceExistsForHost ensures that there is a GitService CRD for the given host and kind
func EnsureGitServiceExistsForHost(jxClient versioned.Interface, devNs string, kind string, name string, gitUrl string, out io.Writer) error {
	if kind == "" || kind == "github" || gitUrl == "" {
		return nil
	}

	gitServices := jxClient.JenkinsV1().GitServices(devNs)
	list, err := gitServices.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, gs := range list.Items {
		if gs.Spec.URL == gitUrl {
			oldKind := gs.Spec.GitKind
			if oldKind != kind {
				fmt.Fprintf(out, "Updating GitService %s as the kind has changed from %s to %s\n", gs.Name, oldKind, kind)
				gs.Spec.GitKind = kind
				_, err = gitServices.Update(&gs)
				if err != nil {
					return fmt.Errorf("Failed to update kind on GitService with name %s: %s", gs.Name, err)
				}
				return err
			} else {
				return nil
			}
		}
	}
	if name == "" {
		u, err := url.Parse(gitUrl)
		if err != nil {
			return fmt.Errorf("No name supplied and could not parse URL %s due to %s", u, err)
		}
		name = u.Host
	}

	// not found so lets create a new GitService
	gitSvc := &v1.GitService{
		ObjectMeta: metav1.ObjectMeta{
			Name: ToValidNameWithDots(name),
		},
		Spec: v1.GitServiceSpec{
			Name:    name,
			URL:     gitUrl,
			GitKind: kind,
		},
	}
	current, err := gitServices.Get(name, metav1.GetOptions{})
	if err != nil {
		_, err = gitServices.Create(gitSvc)
		if err != nil {
			return fmt.Errorf("Failed to create GitService with name %s: %s", gitSvc.Name, err)
		}
	} else if current != nil {
		if current.Spec.URL != gitSvc.Spec.URL || current.Spec.GitKind != gitSvc.Spec.GitKind {
			current.Spec.URL = gitSvc.Spec.URL
			current.Spec.GitKind = gitSvc.Spec.GitKind

			_, err = gitServices.Update(current)
			if err != nil {
				return fmt.Errorf("Failed to update GitService with name %s: %s", gitSvc.Name, err)
			}
		}
	}
	return nil
}

// GetGitServiceKind returns the kind of the given host if one can be found or ""
func GetGitServiceKind(jxClient versioned.Interface, kubeClient kubernetes.Interface, devNs string, gitServiceURL string) (string, error) {
	answer := gits.SaasGitKind(gitServiceURL)
	if answer != "" {
		return answer, nil
	}

	answer, err := getServiceKindFromSecrets(kubeClient, devNs, gitServiceURL)
	if err == nil && answer != "" {
		return answer, nil
	}

	return getServiceKindFromGitServices(jxClient, devNs, gitServiceURL)
}

func getServiceKindFromSecrets(kubeClient kubernetes.Interface, ns string, gitServiceURL string) (string, error) {
	secretList, err := kubeClient.CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to list the secrets")
	}

	for _, secret := range secretList.Items {
		if strings.HasPrefix(secret.GetName(), SecretJenkinsPipelineGitCredentials) {
			annotations := secret.GetAnnotations()
			url, ok := annotations[AnnotationURL]
			if !ok {
				continue
			}
			if url == gitServiceURL {
				labels := secret.GetLabels()
				serviceKind, ok := labels[LabelServiceKind]
				if !ok {
					return "", fmt.Errorf("no service kind label found on secret '%s' for git service '%s'",
						secret.GetName(), gitServiceURL)
				}
				return serviceKind, nil
			}
		}
	}
	return "", fmt.Errorf("no secret found with configuration for '%s' git service", gitServiceURL)
}

func getServiceKindFromGitServices(jxClient versioned.Interface, ns string, gitServiceURL string) (string, error) {
	gitServices := jxClient.JenkinsV1().GitServices(ns)
	list, err := gitServices.List(metav1.ListOptions{})
	if err == nil {
		for _, gs := range list.Items {
			if gs.Spec.URL == gitServiceURL {
				return gs.Spec.GitKind, nil
			}
		}
	}
	return "", fmt.Errorf("no git service resource found with URL '%s'", gitServiceURL)
}
