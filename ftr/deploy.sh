go run . pack . -C ftr-manager
go run . up ftr-manager*.sqar JFtR/ftr-manager
rm ftr-manager*.sqar
go run . pack . -U ftr-manager
go run . up ftr-manager*.fsdl JFtR/ftr-manager
rm ftr-manager*.fsdl
go run . query JFtR/ftr-manager