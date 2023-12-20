#!/bin/sh
# run go generate on .go files under source control; group by dir (package).
unset -v progdir
case "${0}" in
*/*) progdir="${0%/*}";;
*) progdir=.;;
esac
git grep -l '^//go:generate ' -- '*.go' | \
	"${progdir}/xargs_by_dir.sh" go generate -v -x
patch -p1 -N << "ENDEND"
diff --git a/pkg/basic/openapi.go b/pkg/basic/openapi.go
index ae14eb4..4e00e54 100644
--- a/pkg/basic/openapi.go
+++ b/pkg/basic/openapi.go
@@ -1559,7 +1559,7 @@ func (response Compute200JSONResponse) VisitComputeResponse(w http.ResponseWrite
 	w.Header().Set("Content-Type", "application/json")
 	w.WriteHeader(200)
 
-	return json.NewEncoder(w).Encode(response)
+	return json.NewEncoder(w).Encode(response.union)
 }
 
 type Compute400JSONResponse struct{ InvalidRequestJSONResponse }
ENDEND
