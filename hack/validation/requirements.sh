# Requirements Validation

function injectDomainRequirementRestrictions() {
    domain=$1
    rule="self in [\"${domain}/ec2nodeclass\", \"${domain}/instance-encryption-in-transit-supported\", \"${domain}/instance-category\", \"${domain}/instance-hypervisor\", \"${domain}/instance-family\", \"${domain}/instance-generation\", \"${domain}/instance-local-nvme\", \"${domain}/instance-size\", \"${domain}/instance-cpu\", \"${domain}/instance-cpu-manufacturer\", \"${domain}/instance-cpu-sustained-clock-speed-mhz\", \"${domain}/instance-memory\", \"${domain}/instance-ebs-bandwidth\", \"${domain}/instance-network-bandwidth\", \"${domain}/instance-gpu-name\", \"${domain}/instance-gpu-manufacturer\", \"${domain}/instance-gpu-count\", \"${domain}/instance-gpu-memory\", \"${domain}/instance-accelerator-name\", \"${domain}/instance-accelerator-manufacturer\", \"${domain}/instance-accelerator-count\"] || !self.find(\"^([^/]+)\").endsWith(\"${domain}\")"
    message="label domain \"${domain}\" is restricted"
    MSG="${message}" RULE="${rule}" yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.requirements.items.properties.key.x-kubernetes-validations += [{"message": strenv(MSG), "rule": strenv(RULE)}]' -i pkg/apis/crds/karpenter.sh_nodeclaims.yaml
    MSG="${message}" RULE="${rule}" yq eval '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.template.properties.spec.properties.requirements.items.properties.key.x-kubernetes-validations += [{"message": strenv(MSG), "rule": strenv(RULE)}]' -i pkg/apis/crds/karpenter.sh_nodepools.yaml
}
