/*
Pulp 3 API

Fetch, Upload, Organize, and Distribute Software Packages

API version: v3
Contact: pulp-list@redhat.com
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package pulpclient

import (
	"encoding/json"
)

// checks if the DebPackageIndex type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &DebPackageIndex{}

// DebPackageIndex A serializer for PackageIndex.
type DebPackageIndex struct {
	// A URI of a repository the new content unit should be associated with.
	Repository *string `json:"repository,omitempty"`
	// A dict mapping relative paths inside the Content to the correspondingArtifact URLs. E.g.: {'relative/path': '/artifacts/1/'
	Artifacts map[string]interface{} `json:"artifacts"`
	// Component of the component - architecture combination.
	Component *string `json:"component,omitempty"`
	// Architecture of the component - architecture combination.
	Architecture *string `json:"architecture,omitempty"`
	// Path of file relative to url.
	RelativePath *string `json:"relative_path,omitempty"`
	AdditionalProperties map[string]interface{}
}

type _DebPackageIndex DebPackageIndex

// NewDebPackageIndex instantiates a new DebPackageIndex object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewDebPackageIndex(artifacts map[string]interface{}) *DebPackageIndex {
	this := DebPackageIndex{}
	this.Artifacts = artifacts
	return &this
}

// NewDebPackageIndexWithDefaults instantiates a new DebPackageIndex object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewDebPackageIndexWithDefaults() *DebPackageIndex {
	this := DebPackageIndex{}
	return &this
}

// GetRepository returns the Repository field value if set, zero value otherwise.
func (o *DebPackageIndex) GetRepository() string {
	if o == nil || IsNil(o.Repository) {
		var ret string
		return ret
	}
	return *o.Repository
}

// GetRepositoryOk returns a tuple with the Repository field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *DebPackageIndex) GetRepositoryOk() (*string, bool) {
	if o == nil || IsNil(o.Repository) {
		return nil, false
	}
	return o.Repository, true
}

// HasRepository returns a boolean if a field has been set.
func (o *DebPackageIndex) HasRepository() bool {
	if o != nil && !IsNil(o.Repository) {
		return true
	}

	return false
}

// SetRepository gets a reference to the given string and assigns it to the Repository field.
func (o *DebPackageIndex) SetRepository(v string) {
	o.Repository = &v
}

// GetArtifacts returns the Artifacts field value
func (o *DebPackageIndex) GetArtifacts() map[string]interface{} {
	if o == nil {
		var ret map[string]interface{}
		return ret
	}

	return o.Artifacts
}

// GetArtifactsOk returns a tuple with the Artifacts field value
// and a boolean to check if the value has been set.
func (o *DebPackageIndex) GetArtifactsOk() (map[string]interface{}, bool) {
	if o == nil {
		return map[string]interface{}{}, false
	}
	return o.Artifacts, true
}

// SetArtifacts sets field value
func (o *DebPackageIndex) SetArtifacts(v map[string]interface{}) {
	o.Artifacts = v
}

// GetComponent returns the Component field value if set, zero value otherwise.
func (o *DebPackageIndex) GetComponent() string {
	if o == nil || IsNil(o.Component) {
		var ret string
		return ret
	}
	return *o.Component
}

// GetComponentOk returns a tuple with the Component field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *DebPackageIndex) GetComponentOk() (*string, bool) {
	if o == nil || IsNil(o.Component) {
		return nil, false
	}
	return o.Component, true
}

// HasComponent returns a boolean if a field has been set.
func (o *DebPackageIndex) HasComponent() bool {
	if o != nil && !IsNil(o.Component) {
		return true
	}

	return false
}

// SetComponent gets a reference to the given string and assigns it to the Component field.
func (o *DebPackageIndex) SetComponent(v string) {
	o.Component = &v
}

// GetArchitecture returns the Architecture field value if set, zero value otherwise.
func (o *DebPackageIndex) GetArchitecture() string {
	if o == nil || IsNil(o.Architecture) {
		var ret string
		return ret
	}
	return *o.Architecture
}

// GetArchitectureOk returns a tuple with the Architecture field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *DebPackageIndex) GetArchitectureOk() (*string, bool) {
	if o == nil || IsNil(o.Architecture) {
		return nil, false
	}
	return o.Architecture, true
}

// HasArchitecture returns a boolean if a field has been set.
func (o *DebPackageIndex) HasArchitecture() bool {
	if o != nil && !IsNil(o.Architecture) {
		return true
	}

	return false
}

// SetArchitecture gets a reference to the given string and assigns it to the Architecture field.
func (o *DebPackageIndex) SetArchitecture(v string) {
	o.Architecture = &v
}

// GetRelativePath returns the RelativePath field value if set, zero value otherwise.
func (o *DebPackageIndex) GetRelativePath() string {
	if o == nil || IsNil(o.RelativePath) {
		var ret string
		return ret
	}
	return *o.RelativePath
}

// GetRelativePathOk returns a tuple with the RelativePath field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *DebPackageIndex) GetRelativePathOk() (*string, bool) {
	if o == nil || IsNil(o.RelativePath) {
		return nil, false
	}
	return o.RelativePath, true
}

// HasRelativePath returns a boolean if a field has been set.
func (o *DebPackageIndex) HasRelativePath() bool {
	if o != nil && !IsNil(o.RelativePath) {
		return true
	}

	return false
}

// SetRelativePath gets a reference to the given string and assigns it to the RelativePath field.
func (o *DebPackageIndex) SetRelativePath(v string) {
	o.RelativePath = &v
}

func (o DebPackageIndex) MarshalJSON() ([]byte, error) {
	toSerialize,err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o DebPackageIndex) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Repository) {
		toSerialize["repository"] = o.Repository
	}
	toSerialize["artifacts"] = o.Artifacts
	if !IsNil(o.Component) {
		toSerialize["component"] = o.Component
	}
	if !IsNil(o.Architecture) {
		toSerialize["architecture"] = o.Architecture
	}
	if !IsNil(o.RelativePath) {
		toSerialize["relative_path"] = o.RelativePath
	}

	for key, value := range o.AdditionalProperties {
		toSerialize[key] = value
	}

	return toSerialize, nil
}

func (o *DebPackageIndex) UnmarshalJSON(bytes []byte) (err error) {
	varDebPackageIndex := _DebPackageIndex{}

	if err = json.Unmarshal(bytes, &varDebPackageIndex); err == nil {
		*o = DebPackageIndex(varDebPackageIndex)
	}

	additionalProperties := make(map[string]interface{})

	if err = json.Unmarshal(bytes, &additionalProperties); err == nil {
		delete(additionalProperties, "repository")
		delete(additionalProperties, "artifacts")
		delete(additionalProperties, "component")
		delete(additionalProperties, "architecture")
		delete(additionalProperties, "relative_path")
		o.AdditionalProperties = additionalProperties
	}

	return err
}

type NullableDebPackageIndex struct {
	value *DebPackageIndex
	isSet bool
}

func (v NullableDebPackageIndex) Get() *DebPackageIndex {
	return v.value
}

func (v *NullableDebPackageIndex) Set(val *DebPackageIndex) {
	v.value = val
	v.isSet = true
}

func (v NullableDebPackageIndex) IsSet() bool {
	return v.isSet
}

func (v *NullableDebPackageIndex) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableDebPackageIndex(val *DebPackageIndex) *NullableDebPackageIndex {
	return &NullableDebPackageIndex{value: val, isSet: true}
}

func (v NullableDebPackageIndex) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableDebPackageIndex) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}

