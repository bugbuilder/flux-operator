// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/kustomize"
	. "github.com/onsi/gomega"
	cp "github.com/otiai10/copy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestBuild(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version
	options.Shards = []string{"shard1", "shard2"}
	options.Patches = ProfileOpenShift + GetMultitenantProfile("")
	options.ArtifactStorage = &ArtifactStorage{
		Class: "standard",
		Size:  "10Gi",
	}

	srcDir := filepath.Join("testdata", version)
	dstDir := filepath.Join("testdata", "output")
	err := os.RemoveAll(dstDir)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())
}

func TestBuild_Defaults(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version

	srcDir := filepath.Join("testdata", version)
	goldenFile := filepath.Join("testdata", version+"-golden", "default.kustomization.yaml")

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())

	if shouldGenGolden() {
		err = cp.Copy(filepath.Join(dstDir, "kustomization.yaml"), goldenFile)
		g.Expect(err).NotTo(HaveOccurred())
	}

	genK, err := os.ReadFile(filepath.Join(dstDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(genK)).To(Equal(string(goldenK)))
}

func TestBuild_Patches(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version

	srcDir := filepath.Join("testdata", version)
	goldenFile := filepath.Join("testdata", version+"-golden", "patches.kustomization.yaml")

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	patches := []kustomize.Patch{
		{
			Patch: `
- op: remove
  path: /metadata/labels/pod-security.kubernetes.io~1warn
- op: remove
  path: /metadata/labels/pod-security.kubernetes.io~1warn-version
`,
			Target: &kustomize.Selector{
				Kind: "Namespace",
			},
		},
	}
	patchesData, err := yaml.Marshal(patches)
	g.Expect(err).NotTo(HaveOccurred())
	options.Patches = string(patchesData)

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())
	g.Expect(result.Revision).To(HavePrefix(version + "@sha256:"))

	if shouldGenGolden() {
		err = cp.Copy(filepath.Join(dstDir, "kustomization.yaml"), goldenFile)
		g.Expect(err).NotTo(HaveOccurred())
	}

	genK, err := os.ReadFile(filepath.Join(dstDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(genK)).To(Equal(string(goldenK)))

	found := false
	for _, obj := range result.Objects {
		if obj.GetKind() == "Namespace" {
			found = true
			labels := obj.GetLabels()
			g.Expect(labels).NotTo(HaveKey("pod-security.kubernetes.io/warn"))
			g.Expect(labels).NotTo(HaveKey("pod-security.kubernetes.io/warn-version"))
			g.Expect(obj.GetAnnotations()).To(HaveKeyWithValue("fluxcd.controlplane.io/prune", "disabled"))
		}
	}
	g.Expect(found).To(BeTrue())
}

func TestBuild_Profiles(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version

	srcDir := filepath.Join("testdata", version)
	goldenFile := filepath.Join("testdata", version+"-golden", "profiles.kustomization.yaml")

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	options.Patches = ProfileOpenShift + GetMultitenantProfile("")

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())
	g.Expect(result.Revision).To(HavePrefix(version + "@sha256:"))

	if shouldGenGolden() {
		err = cp.Copy(filepath.Join(dstDir, "kustomization.yaml"), goldenFile)
		g.Expect(err).NotTo(HaveOccurred())
	}

	genK, err := os.ReadFile(filepath.Join(dstDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(genK)).To(Equal(string(goldenK)))

	found := false
	for _, obj := range result.Objects {
		labels := obj.GetLabels()
		if obj.GetKind() == "Namespace" {
			found = true
			g.Expect(labels).NotTo(HaveKey("pod-security.kubernetes.io/warn"))
			g.Expect(labels).NotTo(HaveKey("pod-security.kubernetes.io/warn-version"))
		}
		g.Expect(obj.GetLabels()).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "flux-operator"))
	}
	g.Expect(found).To(BeTrue())
}

func TestBuild_ArtifactStorage(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version

	srcDir := filepath.Join("testdata", version)
	goldenFile := filepath.Join("testdata", version+"-golden", "storage.kustomization.yaml")

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	options.ArtifactStorage = &ArtifactStorage{
		Class: "standard",
		Size:  "10Gi",
	}

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())
	g.Expect(result.Revision).To(HavePrefix(version + "@sha256:"))

	if shouldGenGolden() {
		err = cp.Copy(filepath.Join(dstDir, "kustomization.yaml"), goldenFile)
		g.Expect(err).NotTo(HaveOccurred())
	}

	genK, err := os.ReadFile(filepath.Join(dstDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(genK)).To(Equal(string(goldenK)))

	found := false
	for _, obj := range result.Objects {
		if obj.GetKind() == "PersistentVolumeClaim" {
			found = true
			g.Expect(obj.GetName()).To(Equal("source-controller"))
		}
		g.Expect(obj.GetAnnotations()).To(HaveKeyWithValue("kustomize.toolkit.fluxcd.io/ssa", "Ignore"))
	}
	g.Expect(found).To(BeTrue())
}

func TestBuild_Sync(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version

	srcDir := filepath.Join("testdata", version)
	goldenFile := filepath.Join("testdata", version+"-golden", "sync.kustomization.yaml")

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	options.Sync = &Sync{
		Name:     "flux-system",
		Interval: "5m",
		Kind:     "GitRepository",
		URL:      "https://host/repo.git",
		Ref:      "refs/heads/main",
		Path:     "clusters/prod",
		Provider: "github",
	}

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())
	g.Expect(result.Revision).To(HavePrefix(version + "@sha256:"))

	if shouldGenGolden() {
		err = cp.Copy(filepath.Join(dstDir, "kustomization.yaml"), goldenFile)
		g.Expect(err).NotTo(HaveOccurred())
	}

	genK, err := os.ReadFile(filepath.Join(dstDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(genK)).To(Equal(string(goldenK)))

	found := false
	for _, obj := range result.Objects {
		if obj.GetKind() == "GitRepository" || obj.GetKind() == "Kustomization" {
			found = true
			g.Expect(obj.GetName()).To(Equal(options.Namespace))
		}
		g.Expect(obj.GetAnnotations()).To(HaveKeyWithValue("kustomize.toolkit.fluxcd.io/ssa", "Ignore"))
	}
	g.Expect(found).To(BeTrue())

	for _, obj := range result.Objects {
		if obj.GetKind() == "GitRepository" {
			p, _, _ := unstructured.NestedString(obj.Object, "spec", "provider")
			g.Expect(p).To(Equal("github"))
		}
	}
}

func TestBuild_Sync_OCIRepository(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.6.0"
	options := MakeDefaultOptions()
	options.Version = version

	srcDir := filepath.Join("testdata", version)

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	options.Sync = &Sync{
		Name:     "flux-system",
		Interval: "5m",
		Kind:     "OCIRepository",
		URL:      "oci://registry/artifact",
		Ref:      "latest",
		Path:     "clusters/prod",
	}

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())

	found := false
	for _, obj := range result.Objects {
		if obj.GetKind() == "OCIRepository" {
			found = true
			g.Expect(obj.GetAPIVersion()).To(Equal("source.toolkit.fluxcd.io/v1"))
			p, _, _ := unstructured.NestedString(obj.Object, "spec", "ref", "tag")
			g.Expect(p).To(Equal("latest"))
		}
	}
	g.Expect(found).To(BeTrue())
}

func TestBuild_Sync_Bucket(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version

	srcDir := filepath.Join("testdata", version)

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	options.Sync = &Sync{
		Name:     "flux-system",
		Interval: "5m",
		Kind:     "Bucket",
		URL:      "minio.my-org.com",
		Ref:      "my-bucket",
		Path:     "clusters/prod",
	}

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())

	found := false
	for _, obj := range result.Objects {
		if obj.GetKind() == "Bucket" {
			found = true
			g.Expect(obj.GetAPIVersion()).To(Equal("source.toolkit.fluxcd.io/v1beta2"))
			p, _, _ := unstructured.NestedString(obj.Object, "spec", "bucketName")
			g.Expect(p).To(Equal("my-bucket"))
		}
	}
	g.Expect(found).To(BeTrue())
}

func TestBuild_InvalidPatches(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version
	srcDir := filepath.Join("testdata", version)

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	patches := []kustomize.Patch{
		{
			Patch: `
- op: removes
  path: /metadata/labels/pod-security.kubernetes.io~1warn
`,
			Target: &kustomize.Selector{
				Kind: "Namespace",
			},
		},
	}
	patchesData, err := yaml.Marshal(patches)
	g.Expect(err).NotTo(HaveOccurred())
	options.Patches = string(patchesData)

	_, err = Build(srcDir, dstDir, options)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("Unexpected kind: removes"))
}

func TestBuild_Sharding(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	options := MakeDefaultOptions()
	options.Version = version
	options.Shards = []string{"shard1", "shard2"}
	options.Patches = ProfileOpenShift + GetMultitenantProfile("")
	options.ArtifactStorage = &ArtifactStorage{
		Class: "standard",
		Size:  "10Gi",
	}

	srcDir := filepath.Join("testdata", version)
	goldenFile := filepath.Join("testdata", version+"-golden", "sharding.kustomization.yaml")

	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())

	if shouldGenGolden() {
		err = cp.Copy(filepath.Join(dstDir, "kustomization.yaml"), goldenFile)
		g.Expect(err).NotTo(HaveOccurred())
	}

	genK, err := os.ReadFile(filepath.Join(dstDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(genK)).To(Equal(string(goldenK)))

	found := false
	for _, obj := range result.Objects {
		if strings.Contains(obj.GetName(), options.Shards[0]) {
			found = true
			g.Expect(obj.GetAnnotations()).To(HaveKeyWithValue("sharding.fluxcd.io/role", "shard"))
		}
	}
	g.Expect(found).To(BeTrue())
}

func testTempDir(t *testing.T) (string, error) {
	tmpDir := t.TempDir()

	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return "", fmt.Errorf("error evaluating symlink: '%w'", err)
	}

	return tmpDir, err
}

func shouldGenGolden() bool {
	return os.Getenv("GEN_GOLDEN") == "true"
}
