package pwr

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/itchio/go-brotli/enc"
	"github.com/itchio/savior"
	"github.com/itchio/savior/brotlisource"
	"github.com/itchio/savior/seeksource"
	"github.com/itchio/wharf/bsdiff"
	"github.com/itchio/wharf/pools/fspool"
	"github.com/itchio/wharf/state"
	"github.com/itchio/wharf/tlc"
	"github.com/itchio/wharf/wtest"
)

type brotliCompressor struct{}

func (bc *brotliCompressor) Apply(writer io.Writer, quality int32) (io.Writer, error) {
	return enc.NewBrotliWriter(writer, &enc.BrotliWriterOptions{
		Quality: int(quality),
	}), nil
}

type brotliDecompressor struct{}

func (bc *brotliDecompressor) Apply(source savior.Source) (savior.Source, error) {
	return brotlisource.New(source), nil
}

func init() {
	RegisterCompressor(CompressionAlgorithm_BROTLI, &brotliCompressor{})
	RegisterDecompressor(CompressionAlgorithm_BROTLI, &brotliDecompressor{})
}

func Test_RediffOneSeq(t *testing.T) {
	runRediffScenario(t, patchScenario{
		name:         "rediff when one byte gets changed",
		touchedFiles: 1,
		deletedFiles: 0,
		v1: testDirSettings{
			entries: []testDirEntry{
				{path: "file", data: []byte{
					0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19,
				}},
			},
		},
		v2: testDirSettings{
			entries: []testDirEntry{
				{path: "file", data: []byte{
					0x00,
					0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09,
					0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19,
				}},
			},
		},
	})
}

func Test_RediffOneInt(t *testing.T) {
	runRediffScenario(t, patchScenario{
		name:         "rediff when one byte gets changed",
		touchedFiles: 1,
		deletedFiles: 0,
		v1: testDirSettings{
			entries: []testDirEntry{
				{path: "file", data: []byte{
					0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19,
				}},
			},
		},
		v2: testDirSettings{
			entries: []testDirEntry{
				{path: "file", data: []byte{
					0x00,
					0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09,
					0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19,
				}},
			},
		},
		partitions: 2,
	})
}

func Test_RediffWorse(t *testing.T) {
	runRediffScenario(t, patchScenario{
		name:         "rediff gets slightly worse",
		touchedFiles: 1,
		deletedFiles: 0,
		v1: testDirSettings{
			entries: []testDirEntry{
				{path: "subdir/file-1", seed: 0x1, size: BlockSize*2 + 14},
				{path: "file-1", seed: 0x2},
				{path: "dir2/file-2", seed: 0x3},
			},
		},
		v2: testDirSettings{
			entries: []testDirEntry{
				{path: "subdir/file-1", seed: 0x1, size: BlockSize*3 + 14},
				{path: "file-1", seed: 0x2},
				{path: "dir2/file-2", seed: 0x33},
			},
		},
	})
}

func Test_RediffBetter(t *testing.T) {
	for _, partitions := range []int{0, 2, 4, 8} {
		// for _, partitions := range []int{8} {
		runRediffScenario(t, patchScenario{
			name:         "rediff gets better!",
			touchedFiles: 1,
			deletedFiles: 0,
			v1: testDirSettings{
				entries: []testDirEntry{
					{path: "subdir/file-1", seed: 0x1, size: BlockSize*5 + 14},
					{path: "file-1", seed: 0x2},
					{path: "dir2/file-2", seed: 0x3},
				},
			},
			v2: testDirSettings{
				entries: []testDirEntry{
					{path: "subdir/file-1", seed: 0x1, size: BlockSize*5 + 14, bsmods: []bsmod{
						bsmod{interval: BlockSize/2 + 3, delta: 0x4},
						bsmod{interval: BlockSize/3 + 7, delta: 0x18},
					}},
					{path: "file-1", seed: 0x2},
					{path: "dir2/file-2", seed: 0x33},
				},
			},
			partitions: partitions,
		})
	}
}

func Test_RediffEdgeCases(t *testing.T) {
	for _, partitions := range []int{0, 2, 4, 8} {
		runRediffScenario(t, patchScenario{
			name:         "rediff gets better!",
			touchedFiles: 1,
			deletedFiles: 0,
			v1: testDirSettings{
				entries: []testDirEntry{
					{path: "file1", seed: 0x1, size: 0},
					{path: "file2", seed: 0x2},
					{path: "file3", seed: 0x3, size: 1},
					{path: "file4", seed: 0x5, size: 8},
					{path: "file5", seed: 0x7, size: 9},
				},
			},
			v2: testDirSettings{
				entries: []testDirEntry{
					{path: "file1", seed: 0x1},
					{path: "file2", seed: 0x2, size: 0},
					{path: "file3", seed: 0x4, size: 1},
					{path: "file5", seed: 0x6, size: 8},
					{path: "file6", seed: 0x8, size: 9},
				},
			},
			partitions: partitions,
		})
	}
}

func Test_RediffStillBetter(t *testing.T) {
	runRediffScenario(t, patchScenario{
		name:         "rediff gets better even though rsync wasn't that bad",
		touchedFiles: 1,
		deletedFiles: 0,
		v1: testDirSettings{
			entries: []testDirEntry{
				{path: "subdir/file-1", seed: 0x1, size: BlockSize*5 + 14},
				{path: "file-1", seed: 0x2, size: BlockSize * 4},
				{path: "dir2/file-2", seed: 0x3},
			},
		},
		v2: testDirSettings{
			entries: []testDirEntry{
				{path: "subdir/file-1", seed: 0x1, size: BlockSize * 6, bsmods: []bsmod{
					bsmod{interval: BlockSize/7 + 3, delta: 0x4, max: 4, skip: 20},
					bsmod{interval: BlockSize/13 + 7, delta: 0x18, max: 6, skip: 20},
				}},
				{path: "file-1", chunks: []testDirChunk{
					testDirChunk{size: BlockSize*2 + 3, seed: 0x99},
					testDirChunk{size: BlockSize*1 + 12, seed: 0x2},
				}},
				{path: "dir2/file-2", seed: 0x33},
			},
		},
	})
}

func runRediffScenario(t *testing.T, scenario patchScenario) {
	log := t.Logf

	mainDir, err := ioutil.TempDir("", "rediff")
	wtest.Must(t, err)
	wtest.Must(t, os.MkdirAll(mainDir, 0755))
	defer os.RemoveAll(mainDir)

	v1 := filepath.Join(mainDir, "v1")
	makeTestDir(t, v1, scenario.v1)

	v2 := filepath.Join(mainDir, "v2")
	makeTestDir(t, v2, scenario.v2)

	compression := &CompressionSettings{}
	compression.Algorithm = CompressionAlgorithm_BROTLI
	compression.Quality = 1

	sourceContainer, err := tlc.WalkAny(v2, &tlc.WalkOpts{})
	wtest.Must(t, err)

	consumer := &state.Consumer{
		OnMessage: func(level string, message string) {
			// log("[%s] %s", level, message)
		},
	}
	patchBuffer := new(bytes.Buffer)
	signatureBuffer := new(bytes.Buffer)

	func() {
		targetContainer, dErr := tlc.WalkAny(v1, &tlc.WalkOpts{})
		wtest.Must(t, dErr)

		targetPool := fspool.New(targetContainer, v1)
		targetSignature, dErr := ComputeSignature(context.Background(), targetContainer, targetPool, consumer)
		wtest.Must(t, dErr)

		log("Diffing %s -> %s",
			humanize.IBytes(uint64(targetContainer.Size)),
			humanize.IBytes(uint64(sourceContainer.Size)),
		)

		pool := fspool.New(sourceContainer, v2)

		dctx := &DiffContext{
			Compression: compression,
			Consumer:    consumer,

			SourceContainer: sourceContainer,
			Pool:            pool,

			TargetContainer: targetContainer,
			TargetSignature: targetSignature,
		}

		wtest.Must(t, dctx.WritePatch(context.Background(), patchBuffer, signatureBuffer))
	}()

	sigReader := seeksource.FromBytes(signatureBuffer.Bytes())
	_, sigErr := sigReader.Resume(nil)
	wtest.Must(t, sigErr)

	signature, sErr := ReadSignature(context.Background(), sigReader)
	wtest.Must(t, sErr)

	v1Before := filepath.Join(mainDir, "v1Before")
	cpDir(t, v1, v1Before)

	v1After := filepath.Join(mainDir, "v1After")

	wtest.Must(t, os.RemoveAll(v1Before))
	cpDir(t, v1, v1Before)

	func() {
		actx := &ApplyContext{
			TargetPath: v1Before,
			OutputPath: v1After,

			Consumer: consumer,
		}

		patchReader := seeksource.FromBytes(patchBuffer.Bytes())
		_, rErr := patchReader.Resume(nil)
		wtest.Must(t, rErr)

		aErr := actx.ApplyPatch(patchReader)
		wtest.Must(t, aErr)

		sigReader := seeksource.FromBytes(signatureBuffer.Bytes())
		_, sigErr := sigReader.Resume(nil)
		wtest.Must(t, sigErr)

		signature, sErr := ReadSignature(context.Background(), sigReader)
		wtest.Must(t, sErr)
		wtest.Must(t, AssertValid(v1After, signature))
		log("Original applies cleanly")
	}()

	func() {
		targetContainer, dErr := tlc.WalkAny(v1, &tlc.WalkOpts{})
		wtest.Must(t, dErr)

		bsdiffStats := &bsdiff.DiffStats{}

		rc := &RediffContext{
			TargetPool: fspool.New(targetContainer, v1),
			SourcePool: fspool.New(sourceContainer, v2),

			Consumer:              consumer,
			Compression:           compression,
			SuffixSortConcurrency: 0,
			Partitions:            scenario.partitions,

			BsdiffStats: bsdiffStats,
		}

		patchReader := seeksource.FromBytes(patchBuffer.Bytes())
		_, rErr := patchReader.Resume(nil)
		wtest.Must(t, rErr)

		log("Optimizing (%d partitions)...", rc.Partitions)
		aErr := rc.AnalyzePatch(patchReader)
		wtest.Must(t, aErr)

		_, rErr = patchReader.Resume(nil)
		wtest.Must(t, rErr)
		optimizedPatchBuffer := new(bytes.Buffer)

		beforeOptimize := time.Now()
		oErr := rc.OptimizePatch(patchReader, optimizedPatchBuffer)
		wtest.Must(t, oErr)
		log("Optimized patch in %s (spent %s sorting, %s scanning)",
			time.Since(beforeOptimize),
			bsdiffStats.TimeSpentSorting,
			bsdiffStats.TimeSpentScanning,
		)

		before := patchBuffer.Len()
		after := optimizedPatchBuffer.Len()

		diff := (float64(after) - float64(before)) / float64(before) * 100
		if diff < 0 {
			log("Patch is %.2f%% smaller (%s < %s)", -diff, humanize.IBytes(uint64(after)), humanize.IBytes(uint64(before)))
		} else {
			log("Patch is %.2f%% larger (%s > %s)", diff, humanize.IBytes(uint64(after)), humanize.IBytes(uint64(before)))
		}

		wtest.Must(t, os.RemoveAll(v1Before))
		cpDir(t, v1, v1Before)

		func() {
			actx := &ApplyContext{
				TargetPath: v1Before,
				OutputPath: v1After,

				Consumer: consumer,
			}

			patchReader := seeksource.FromBytes(optimizedPatchBuffer.Bytes())
			_, rErr := patchReader.Resume(nil)
			wtest.Must(t, rErr)

			aErr := actx.ApplyPatch(patchReader)
			wtest.Must(t, aErr)

			wtest.Must(t, AssertValid(v1After, signature))
			log("Optimized patch applies cleanly.")
		}()
	}()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
