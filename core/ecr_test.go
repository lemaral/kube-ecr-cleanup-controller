package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
)

type mockECRClient struct {
	t *testing.T
	ecriface.ECRAPI

	expectedListRepositoryNames []string

	outputError error
}

func (m *mockECRClient) DescribeRepositoriesPages(input *ecr.DescribeRepositoriesInput, fn func(*ecr.DescribeRepositoriesOutput, bool) bool) error {
	if input == nil {
		m.t.Errorf("Unexpected nil input")
	}

	if len(input.RepositoryNames) != len(m.expectedListRepositoryNames) {
		m.t.Errorf("Expected number of repository names was %d, but the actual value was %d", len(m.expectedListRepositoryNames), len(input.RepositoryNames))
	}

	for i := range input.RepositoryNames {
		if *input.RepositoryNames[i] != m.expectedListRepositoryNames[i] {
			m.t.Errorf("Expected repository name in idx %d to be %v, but was %v", i, m.expectedListRepositoryNames[i], *input.RepositoryNames[i])
		}
	}

	repositoryName := "repo-name"
	page := &ecr.DescribeRepositoriesOutput{
		Repositories: []*ecr.Repository{
			{
				RepositoryName: &repositoryName,
			},
		},
	}

	// There's two pages, so the function must return true
	if fn(page, false) != true {
		m.t.Errorf("Expected callback to return true for first page, but returned false")
	}

	// This is the last page, so the function must return false
	if fn(page, true) != false {
		m.t.Errorf("Expected callback to return true for last page, but returned true")
	}

	return m.outputError
}

func TestSortImagesByPushDate(t *testing.T) {
	orderedTime := []time.Time{
		time.Unix(0, 0),
		time.Unix(1, 0),
		time.Unix(2, 0),
	}

	ecrImages := []*ecr.ImageDetail{
		{
			ImagePushedAt: &orderedTime[2],
		},
		{
			ImagePushedAt: &orderedTime[1],
		},
		{
			ImagePushedAt: &orderedTime[0],
		},
	}

	SortImagesByPushDate(ecrImages)

	if len(ecrImages) != 3 {
		t.Errorf("Expected image list to remain with 3 elements, but the size is now %d", len(ecrImages))
	}

	for i := range ecrImages {
		actualDate := *ecrImages[i].ImagePushedAt
		expectedDate := time.Unix(int64(i), 0)

		if *ecrImages[i].ImagePushedAt != expectedDate {
			t.Errorf("Expected ercImages[%d] timestamp to be %+v, but was %+v", i, expectedDate, actualDate)
		}
	}
}

func TestListRepositoriesWithEmptyRepos(t *testing.T) {
	client := ECRClientImpl{
		ECRClient: nil, // Should not interact with the ECR client
	}

	repos, err := client.ListRepositories([]*string{})

	if len(repos) != 0 {
		t.Errorf("Expected repos to be empty, but was not: %q", repos)
	}

	if err != nil {
		t.Errorf("Expected error to be nil, but was %v", err)
	}
}

func TestListRepositoriesError(t *testing.T) {
	repoNames := []string{"repo-1"}

	client := ECRClientImpl{
		ECRClient: &mockECRClient{
			t: t,

			expectedListRepositoryNames: repoNames,

			outputError: fmt.Errorf(""),
		},
	}

	repos, err := client.ListRepositories([]*string{&repoNames[0]})

	if repos != nil {
		t.Errorf("Expected repos to be nil, but was %v", repos)
	}

	if err == nil {
		t.Errorf("Expected error not to be nil, but it was")
	}
}

func TestListRepositories(t *testing.T) {
	repoNames := []string{"repo-1"}

	client := ECRClientImpl{
		ECRClient: &mockECRClient{
			t: t,

			expectedListRepositoryNames: repoNames,
		},
	}

	repos, err := client.ListRepositories([]*string{&repoNames[0]})

	if err != nil {
		t.Errorf("Expected error to be nil, but it was: %v", err)
	}

	if repos == nil {
		t.Errorf("Expected repos not to be nil, but it was")
	}

	if len(repos) != 2 {
		t.Errorf("Expected repos to contain 2 items, but it contains: %q", repos)
	}
}

func TestFilterOldUnusedImages(t *testing.T) {
	tags := []string{"tag-1", "tag-2", "tag-3", "tag-4", "tag-5"}

	orderedTime := []time.Time{
		time.Unix(0, 0),
		time.Unix(1, 0),
		time.Unix(2, 0),
	}

	testCases := []struct {
		keepMax   int
		tagsInUse []string
		images    []*ecr.ImageDetail
		oldImages []*ecr.ImageDetail
	}{

		// Should return no images
		{
			keepMax:   3,
			tagsInUse: []string{},
			images: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[2],
				},
				{
					ImagePushedAt: &orderedTime[1],
				},
				{
					ImagePushedAt: &orderedTime[0],
				},
			},
			oldImages: []*ecr.ImageDetail{},
		},

		// Should return the oldest image
		{
			keepMax:   2,
			tagsInUse: []string{},
			images: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[2],
				},
				{
					ImagePushedAt: &orderedTime[1],
				},
				{
					ImagePushedAt: &orderedTime[0],
				},
			},
			oldImages: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[0],
				},
			},
		},

		// Should return all images sorted by date
		{
			keepMax:   0,
			tagsInUse: []string{},
			images: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[2],
				},
				{
					ImagePushedAt: &orderedTime[1],
				},
				{
					ImagePushedAt: &orderedTime[0],
				},
			},
			oldImages: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[0],
				},
				{
					ImagePushedAt: &orderedTime[1],
				},
				{
					ImagePushedAt: &orderedTime[2],
				},
			},
		},

		// Should return no images as they're all being used
		{
			keepMax:   0,
			tagsInUse: []string{tags[0], tags[2], tags[4]},
			images: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[2],
					ImageTags:     []*string{&tags[4]},
				},
				{
					ImagePushedAt: &orderedTime[1],
					ImageTags:     []*string{&tags[2]},
				},
				{
					ImagePushedAt: &orderedTime[0],
					ImageTags:     []*string{&tags[0]},
				},
			},
			oldImages: []*ecr.ImageDetail{},
		},

		// Should return all images but the oldest one which is in use
		{
			keepMax:   1,
			tagsInUse: []string{tags[0]},
			images: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[2],
					ImageTags:     []*string{&tags[2]},
				},
				{
					ImagePushedAt: &orderedTime[1],
					ImageTags:     []*string{&tags[1]},
				},
				{
					ImagePushedAt: &orderedTime[0],
					ImageTags:     []*string{&tags[0]},
				},
			},
			oldImages: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[1],
				},
				{
					ImagePushedAt: &orderedTime[2],
				},
			},
		},

		// Should return the newest image as the two oldest ones are in use
		{
			keepMax:   1,
			tagsInUse: []string{tags[0], tags[0], tags[1], tags[1]}, // Duplicate tag must be handled correctly
			images: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[2],
					ImageTags:     []*string{&tags[2], &tags[2]}, // Intentionally inconsistent metadata
				},
				{
					ImagePushedAt: &orderedTime[1],
					ImageTags:     []*string{&tags[1]},
				},
				{
					ImagePushedAt: &orderedTime[0],
					ImageTags:     []*string{&tags[0]},
				},
			},
			oldImages: []*ecr.ImageDetail{
				{
					ImagePushedAt: &orderedTime[2],
				},
			},
		},
	}

	for _, testCase := range testCases {
		filtered := FilterOldUnusedImages(testCase.keepMax, testCase.images, testCase.tagsInUse)

		if len(filtered) != len(testCase.oldImages) {
			t.Errorf("Expected list of old images to have %d items, but it has %d:\n\nExpected: %+v\nActual: %+v", len(testCase.oldImages), len(filtered), testCase.oldImages, filtered)
		}

		for i := range filtered {
			actualDate := *filtered[i].ImagePushedAt
			expectedDate := *testCase.oldImages[i].ImagePushedAt

			if actualDate != expectedDate {
				t.Errorf("Expected filtered[%d] timestamp to be %+v, but was %+v", i, expectedDate, actualDate)
			}
		}
	}
}