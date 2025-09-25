# data/files package directory tree

Use this directory tree to store files used by pipelines, eg. pxetree

To prevent name collisions put them into their own directory under files/
and add them to the embed command in files.go

Use the files like this:

```
import "github.com/osbuild/images/data/files"

var fileDataFS fs.FS = files.Data

f, err := fileDataFS.Open("pxetree/README")
```

