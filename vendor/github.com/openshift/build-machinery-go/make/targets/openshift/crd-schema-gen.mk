include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/golang.mk \
	../../lib/tmp.mk \
	../../targets/openshift/controller-gen.mk \
	../../targets/openshift/yq.mk \
	../../targets/openshift/yaml-patch.mk \
)

# $1 - crd file
# $2 - patch file
define patch-crd-yq
	$(YQ) m -i -x '$(1)' '$(2)'

endef

# $1 - crd file
# $2 - patch file
define patch-crd-yaml-patch
	$(YAML_PATCH) -o '$(2)' < '$(1)' > '$(1).patched'
    mv '$(1).patched' '$(1)'

endef

empty :=

# $1 - apis
# $2 - manifests
define run-crd-gen
	'$(CONTROLLER_GEN)' \
		schemapatch:manifests="$(2)" \
		paths="$(subst $(empty) ,;,$(1))" \
<<<<<<< HEAD
		'output:dir="$(2)"'
	$$(foreach p,$$(wildcard $(2)/*.crd.yaml-merge-patch),$$(call patch-crd-yq,$$(basename $$(p)).yaml,$$(p)))
	$$(foreach p,$$(wildcard $(2)/*.crd.yaml-patch),$$(call patch-crd-yaml-patch,$$(basename $$(p)).yaml,$$(p)))
=======
		output:dir="$(3)"
	$$(foreach p,$$(wildcard $(2)/*.crd.yaml-merge-patch),$$(call patch-crd-yq,$$(subst $(2),$(3),$$(basename $$(p))).yaml,$$(p)))
	$$(foreach p,$$(wildcard $(2)/*.crd.yaml-patch),$$(call patch-crd-yaml-patch,$$(subst $(2),$(3),$$(basename $$(p))).yaml,$$(p)))
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
endef


# $1 - target name
# $2 - apis
# $3 - manifests
define add-crd-gen-internal

update-codegen-crds-$(1): ensure-controller-gen ensure-yq ensure-yaml-patch
<<<<<<< HEAD
	$(call run-crd-gen,$(2),$(3))
=======
	$(call run-crd-gen,$(2),$(3),$(4))
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
.PHONY: update-codegen-crds-$(1)

update-codegen-crds: update-codegen-crds-$(1)
.PHONY: update-codegen-crds

<<<<<<< HEAD
verify-codegen-crds-$(1): update-codegen-crds-$(1)
	git diff --exit-code
=======
verify-codegen-crds-$(1): VERIFY_CODEGEN_CRD_TMP_DIR:=$$(shell mktemp -d)
verify-codegen-crds-$(1): ensure-controller-gen ensure-yq ensure-yaml-patch
	$(call run-crd-gen,$(2),$(3),$$(VERIFY_CODEGEN_CRD_TMP_DIR))
	$$(foreach p,$$(wildcard $(4)/*crd.yaml),$$(call diff-file,$$(p),$$(subst $(4),$$(VERIFY_CODEGEN_CRD_TMP_DIR),$$(p))))
>>>>>>> 268252f ( [WIP] Add support ImageDigest,TagMirrorSet CRDs)
.PHONY: verify-codegen-crds-$(1)

verify-codegen-crds: verify-codegen-crds-$(1)
.PHONY: verify-codegen-crds

endef


update-generated: update-codegen-crds
.PHONY: update-generated

update: update-generated
.PHONY: update

verify-generated: verify-codegen-crds
.PHONY: verify-generated

verify: verify-generated
.PHONY: verify


define add-crd-gen
$(eval $(call add-crd-gen-internal,$(1),$(2),$(3)))
endef
