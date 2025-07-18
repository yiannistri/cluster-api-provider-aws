package v1beta2

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager will setup the webhooks for the ROSAMachinePool.
func (r *ROSAMachinePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(rosaMachinePoolWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-rosamachinepool,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=rosamachinepools,versions=v1beta2,name=validation.rosamachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-rosamachinepool,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=rosamachinepools,versions=v1beta2,name=default.rosamachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

type rosaMachinePoolWebhook struct{}

var _ webhook.CustomDefaulter = &rosaMachinePoolWebhook{}
var _ webhook.CustomValidator = &rosaMachinePoolWebhook{}

// ValidateCreate implements admission.Validator.
func (*rosaMachinePoolWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	r, ok := obj.(*ROSAMachinePool)
	if !ok {
		return nil, fmt.Errorf("expected an ROSAMachinePool object but got %T", r)
	}

	var allErrs field.ErrorList

	if err := r.validateVersion(); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := r.validateNodeDrainGracePeriod(); err != nil {
		allErrs = append(allErrs, err)
	}

	allErrs = append(allErrs, r.Spec.AdditionalTags.Validate()...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		r.GroupVersionKind().GroupKind(),
		r.Name,
		allErrs,
	)
}

// ValidateUpdate implements admission.Validator.
func (*rosaMachinePoolWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	r, ok := newObj.(*ROSAMachinePool)
	if !ok {
		return nil, fmt.Errorf("expected an ROSAMachinePool object but got %T", r)
	}

	oldPool, ok := oldObj.(*ROSAMachinePool)
	if !ok {
		return nil, apierrors.NewInvalid(GroupVersion.WithKind("ROSAMachinePool").GroupKind(), r.Name, field.ErrorList{
			field.InternalError(nil, errors.New("failed to convert old ROSAMachinePool to object")),
		})
	}

	var allErrs field.ErrorList
	if err := r.validateVersion(); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := r.validateNodeDrainGracePeriod(); err != nil {
		allErrs = append(allErrs, err)
	}

	allErrs = append(allErrs, validateImmutable(oldPool.Spec.AdditionalSecurityGroups, r.Spec.AdditionalSecurityGroups, "additionalSecurityGroups")...)
	allErrs = append(allErrs, validateImmutable(oldPool.Spec.AdditionalTags, r.Spec.AdditionalTags, "additionalTags")...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		r.GroupVersionKind().GroupKind(),
		r.Name,
		allErrs,
	)
}

// ValidateDelete implements admission.Validator.
func (*rosaMachinePoolWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (r *ROSAMachinePool) validateVersion() *field.Error {
	if r.Spec.Version == "" {
		return nil
	}
	_, err := semver.Parse(r.Spec.Version)
	if err != nil {
		return field.Invalid(field.NewPath("spec.version"), r.Spec.Version, "must be a valid semantic version")
	}

	return nil
}

func (r *ROSAMachinePool) validateNodeDrainGracePeriod() *field.Error {
	if r.Spec.NodeDrainGracePeriod == nil {
		return nil
	}

	if r.Spec.NodeDrainGracePeriod.Minutes() > 10080 {
		return field.Invalid(field.NewPath("spec.nodeDrainGracePeriod"), r.Spec.NodeDrainGracePeriod,
			"max supported duration is 1 week (10080m|168h)")
	}

	return nil
}

func validateImmutable(old, updated interface{}, name string) field.ErrorList {
	var allErrs field.ErrorList

	if !cmp.Equal(old, updated) {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", name), updated, "field is immutable"),
		)
	}

	return allErrs
}

// Default implements admission.Defaulter.
func (*rosaMachinePoolWebhook) Default(ctx context.Context, obj runtime.Object) error {
	r, ok := obj.(*ROSAMachinePool)
	if !ok {
		return fmt.Errorf("expected an ROSAMachinePool object but got %T", r)
	}

	r.Default()
	return nil
}

// Default satisfies the defaulting webhook interface.
func (r *ROSAMachinePool) Default() {
	if r.Spec.NodeDrainGracePeriod == nil {
		r.Spec.NodeDrainGracePeriod = &metav1.Duration{}
	}

	if r.Spec.UpdateConfig == nil {
		r.Spec.UpdateConfig = &RosaUpdateConfig{}
	}
	if r.Spec.UpdateConfig.RollingUpdate == nil {
		r.Spec.UpdateConfig.RollingUpdate = &RollingUpdate{
			MaxUnavailable: ptr.To(intstr.FromInt32(0)),
			MaxSurge:       ptr.To(intstr.FromInt32(1)),
		}
	}
}
