package zuzu

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"strings"
	"time"

	"github.com/xo/zuzu/dataflow"
)

// Loader reads the data that a spec names by url (D25). zuzu reads no file
// and makes no request of its own.
type Loader interface {
	Load(ctx context.Context, req LoadRequest) (io.ReadCloser, error)
}

// LoadRequest says what a spec asks for.
type LoadRequest struct {
	// URL is the url of the spec, as written.
	URL string
	// Format is csv, tsv, json or topojson, from the spec.
	Format string
	// Name is the name of the dataset.
	Name string
}

// LoaderFunc makes a Loader of a function.
type LoaderFunc func(context.Context, LoadRequest) (io.ReadCloser, error)

// Load calls the function.
func (f LoaderFunc) Load(ctx context.Context, req LoadRequest) (io.ReadCloser, error) {
	return f(ctx, req)
}

// FSLoader reads each url as a path in fsys. The caller chooses fsys, so
// zuzu still reads only what the caller allows. A url with a scheme, such
// as https:, is an error.
func FSLoader(fsys fs.FS) Loader {
	return LoaderFunc(func(_ context.Context, req LoadRequest) (io.ReadCloser, error) {
		if i := strings.Index(req.URL, "://"); i >= 0 {
			return nil, fmt.Errorf("loading %q: %w", req.URL, ErrNoLoader)
		}
		f, err := fsys.Open(strings.TrimPrefix(req.URL, "/"))
		if err != nil {
			return nil, fmt.Errorf("loading %q: %w", req.URL, err)
		}
		return f, nil
	})
}

// ResultSet is the result of a query. *sql.Rows satisfies it.
type ResultSet interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// readResultSet reads every row of a result set. A column scans into an
// any, so a NULL is nil, a []byte becomes a string, and an integer becomes
// a float64.
func readResultSet(rs ResultSet) ([]dataflow.Row, error) {
	cols, err := rs.Columns()
	if err != nil {
		return nil, fmt.Errorf("reading the columns of a result set: %w", err)
	}
	var rows []dataflow.Row
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for rs.Next() {
		for i := range vals {
			vals[i] = nil
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scanning a result set: %w", err)
		}
		row := make(dataflow.Row, len(cols))
		for i, c := range cols {
			row[c] = fromSQL(vals[i])
		}
		rows = append(rows, row)
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("reading a result set: %w", err)
	}
	return rows, nil
}

// fromSQL converts a value of a database to a value of an expression.
func fromSQL(v any) any {
	switch x := v.(type) {
	case nil, string, bool, float64, time.Time:
		return x
	case []byte:
		return string(x)
	case sql.RawBytes:
		return string(x)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint())
	case reflect.Float32:
		return rv.Float()
	}
	return fmt.Sprint(v)
}
