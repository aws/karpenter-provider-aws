# Kubelet Validation

# spec.kubelet is an open map in Go, so controller-gen emits an unconstrained
# x-kubernetes-preserve-unknown-fields object for it. This script replaces that with a typed
# schema derived from the k8s.io/kubelet version in go.mod, which is what lets the API server
# reject unknown fields and type errors at apply time instead of at node registration.
# Bumping k8s.io/kubelet and re-running "make verify" is what makes newly released kubelet
# fields available to users; the resulting CRD diff is the review signal for that bump.
#
# Note: rejecting an unknown field requires the request to use Strict field validation.
# kubectl does so by default; clients that don't get the field pruned instead. The
# controller-side check in pkg/apis/v1/kubeletconfiguration_validation.go is the backstop
# for those clients.

CRD=pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml
KUBELET=.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.kubelet
SCHEMA="$(mktemp -t kubeletschema)"
trap 'rm -f "$SCHEMA"' EXIT

# Generated on every run rather than committed as a separate artifact, so the schema can
# never drift from the k8s.io/kubelet version the controller actually compiles against.
go run hack/code/kubeletschema_gen/main.go "$SCHEMA"

# Descriptions are stripped to keep the CRD small: retaining the upstream field docs grows
# ec2nodeclasses.yaml by roughly 47KB rather than 7KB. Upstream docs remain the reference
# for what each field does.
yq eval --inplace 'del(.. | select(has("description")).description)' "$SCHEMA"

# Preserve the description controller-gen produced from the Go field's doc comment, since
# the generated fragment carries no top-level description of its own.
DESC="$(yq eval "${KUBELET}.description" "$CRD")"

yq eval-all --inplace \
  "select(fileIndex==0)${KUBELET} = select(fileIndex==1) | select(fileIndex==0)" \
  "$CRD" "$SCHEMA"
DESC="$DESC" yq eval --inplace "${KUBELET}.description = strenv(DESC)" "$CRD"

# The rules below express constraints the upstream Go types can't: which map keys are
# meaningful, and how fields relate to each other. A typed schema is a prerequisite for
# these -- the API server refuses to compile x-kubernetes-validations against a map marked
# x-kubernetes-preserve-unknown-fields.

# kubeReserved and systemReserved accept only the resource names the kubelet reserves, and a
# reservation can't be negative.
#
# Their values are deliberately not constrained to a resource.Quantity pattern: a value may be a
# CEL expression evaluated per instance type (e.g. "vcpus * 10") rather than a literal quantity,
# and a Quantity pattern would reject it. Karpenter resolves the expression to a quantity before
# it reaches the kubelet, and rejects one that doesn't resolve.
for field in kubeReserved systemReserved; do
  yq eval --inplace "${KUBELET}.properties.${field}.x-kubernetes-validations = [
    {\"message\": \"valid keys for ${field} are ['cpu','memory','ephemeral-storage','pid']\",
     \"rule\": \"self.all(x, x=='cpu' || x=='memory' || x=='ephemeral-storage' || x=='pid')\"},
    {\"message\": \"${field} value cannot be a negative resource quantity\",
     \"rule\": \"self.all(x, !self[x].startsWith('-'))\"}
  ]" "$CRD"
done

# Eviction thresholds are keyed by kubelet eviction signal. evictionHard and evictionSoft
# values may also be percentages, which resource.Quantity itself doesn't accept, so their
# pattern is widened beyond the generated Quantity pattern.
SIGNALS="['memory.available','nodefs.available','nodefs.inodesFree','imagefs.available','imagefs.inodesFree','pid.available']"
for field in evictionHard evictionSoft evictionSoftGracePeriod evictionMinimumReclaim; do
  yq eval --inplace "${KUBELET}.properties.${field}.x-kubernetes-validations = [
    {\"message\": \"valid keys for ${field} are ${SIGNALS}\",
     \"rule\": \"self.all(x, x in ${SIGNALS})\"}
  ]" "$CRD"
  # Only the six signals above are ever valid keys, so the maps are bounded in practice.
  # Declaring that bound keeps the API server's CEL cost estimate for the cross-field
  # evictionSoft/evictionSoftGracePeriod rules below within its budget; without it the
  # estimator assumes unbounded maps and rejects the CRD.
  yq eval --inplace "${KUBELET}.properties.${field}.maxProperties = 6" "$CRD"
done
for field in evictionHard evictionSoft evictionMinimumReclaim; do
  yq eval --inplace "${KUBELET}.properties.${field}.additionalProperties.pattern = \"^((\\d{1,2}(\\.\\d{1,2})?|100(\\.0{1,2})?)%||(\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\\+|-)?(([0-9]+(\\.[0-9]*)?)|(\\.[0-9]+))))?)$\"" "$CRD"
done

# The kubelet ignores a soft eviction threshold that has no grace period, and rejects a
# grace period for a signal it isn't evicting on, so the two maps must have matching keys.
yq eval --inplace "${KUBELET}.x-kubernetes-validations = [
  {\"message\": \"every evictionSoft key must have a matching evictionSoftGracePeriod\",
   \"rule\": \"has(self.evictionSoft) ? self.evictionSoft.all(e, (e in self.?evictionSoftGracePeriod.orValue({}))) : true\"},
  {\"message\": \"every evictionSoftGracePeriod key must have a matching evictionSoft\",
   \"rule\": \"has(self.evictionSoftGracePeriod) ? self.evictionSoftGracePeriod.all(e, (e in self.?evictionSoft.orValue({}))) : true\"},
  {\"message\": \"imageGCHighThresholdPercent must be greater than imageGCLowThresholdPercent\",
   \"rule\": \"(has(self.imageGCHighThresholdPercent) && has(self.imageGCLowThresholdPercent)) ? self.imageGCHighThresholdPercent > self.imageGCLowThresholdPercent : true\"}
]" "$CRD"

# Bounds the upstream Go types don't carry: percentages are 0-100, and pod counts and grace
# periods can't be negative.
for field in imageGCHighThresholdPercent imageGCLowThresholdPercent; do
  yq eval --inplace "${KUBELET}.properties.${field}.minimum = 0" "$CRD"
  yq eval --inplace "${KUBELET}.properties.${field}.maximum = 100" "$CRD"
done
for field in podsPerCore evictionMaxPodGracePeriod; do
  yq eval --inplace "${KUBELET}.properties.${field}.minimum = 0" "$CRD"
done

# maxPods accepts either an integer or a CEL expression string, so a numeric minimum can't be
# used -- it would be applied to the string form too. The bound is expressed as a CEL rule that
# only constrains the integer case; an expression is bounds-checked after it's evaluated.
yq eval --inplace "${KUBELET}.properties.maxPods.x-kubernetes-validations = [
  {\"message\": \"maxPods must be a non-negative integer when set to an integer\",
   \"rule\": \"type(self) != int || self >= 0\"}
]" "$CRD"
