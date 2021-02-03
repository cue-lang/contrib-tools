package ci

import "github.com/SchemaStore/schemastore/src/schemas/json"

workflows: [...{file: string, schema: json.#Workflow}]
workflows: [
	{file: "test.yml", schema: test},
]

#checkoutCode: {
	name: "Checkout code"
	uses: "actions/checkout@v2"
}
#installGo: {
	name: "Install Go"
	uses: "actions/setup-go@v2"
	with: "go-version": "${{ matrix.go_version }}"
}

_#ubuntuLatest: "ubuntu-18.04"
_#latestGo:     "1.15.7"

test: json.#Workflow & {
	name: "Test"
	on: {
		push: branches: ["main"]
		pull_request: branches: ["**"]
	}
	jobs: test: {
		strategy: {
			"fail-fast": false
			matrix: {
				os: [_#ubuntuLatest]
				go_version: [_#latestGo]
			}
		}
		"runs-on": "${{ matrix.os }}"
		steps: [
			#checkoutCode,
			#installGo,
			{
				name: "Verify"
				run:  "go mod verify"
			},
			{
				name: "Generate"
				run:  "go generate ./..."
			},
			{
				name: "Test"
				run:  "go test ./..."
			},
			{
				name: "Race test"
				run:  "go test -race ./..."
				if:   "${{ github.ref == 'main' }}"
			},
			{
				name: "staticcheck"
				run:  "go run honnef.co/go/tools/cmd/staticcheck ./..."
			},
			{
				name: "Tidy"
				run:  "go mod tidy"
			},
			{
				name: "Verify commit is clean"
				run:  #"test -z "$(git status --porcelain)" || (git status; git diff; false)"#
			},
		]
	}
}
