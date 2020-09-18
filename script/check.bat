golint -set_exit_status -min_confidence 0.3 ../...
gocyclo -avg -over 15 ../.
golangci-lint run ../... --config cilint.yml
gosec -quiet -conf "sec.json" -exclude-dir=../temp ../...