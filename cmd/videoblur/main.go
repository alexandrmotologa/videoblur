package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/alexandrmotologa/videoblur/internal/audit"
	"github.com/alexandrmotologa/videoblur/internal/config"
	"github.com/alexandrmotologa/videoblur/internal/domain"
	"github.com/alexandrmotologa/videoblur/internal/media"
	"github.com/alexandrmotologa/videoblur/internal/server"
	"github.com/alexandrmotologa/videoblur/internal/vision"
	"github.com/alexandrmotologa/videoblur/internal/watch"
)

var (
	cfgFile string
	cfg     *config.Config
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "videoblur",
		Short: "Autonomous multimodal video and audio redaction engine",
		Long: `videoblur is a zero-cloud, 100% offline multimodal redaction engine for legal discovery,
law enforcement (police bodycam), and healthcare compliance. It detects and tracks faces,
license plates, and computer screens using Kalman temporal continuity, bleeps spoken PII
using Whisper ASR, and outputs a cryptographically signed compliance manifest.`,
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is videoblur.yaml)")

	rootCmd.AddCommand(newRedactCmd())
	rootCmd.AddCommand(newScanCmd())
	rootCmd.AddCommand(newCertifyCmd())
	rootCmd.AddCommand(newStudioCmd())
	rootCmd.AddCommand(newWatchCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig() *config.Config {
	if cfgFile != "" {
		c, err := config.LoadFromFile(cfgFile)
		if err == nil {
			return c
		}
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config %s: %v. Using defaults.\n", cfgFile, err)
	} else if _, err := os.Stat("videoblur.yaml"); err == nil {
		c, err := config.LoadFromFile("videoblur.yaml")
		if err == nil {
			return c
		}
	}
	return config.DefaultConfig()
}

func newRedactCmd() *cobra.Command {
	var (
		inputPath  string
		outputPath string
		auditPath  string
		mode       string
		sigma      float64
		blockSize  int
		targets    string
		standard   string
		auditor    string
		privKey    string
		pubKey     string
		pdfPath    string
	)

	cmd := &cobra.Command{
		Use:   "redact",
		Short: "Execute full visual and audio redaction pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return fmt.Errorf("input video path required (--input)")
			}

			cfg = loadConfig()

			// Apply CLI flag overrides
			if mode != "" {
				cfg.Visual.DefaultMode = config.RedactionMode(mode)
			}
			if sigma > 0 {
				cfg.Visual.BlurSigma = sigma
			}
			if blockSize > 0 {
				cfg.Visual.MosaicBlockSize = blockSize
			}
			if targets != "" {
				var classes []config.TargetClass
				for _, t := range strings.Split(targets, ",") {
					classes = append(classes, config.TargetClass(strings.TrimSpace(t)))
				}
				cfg.Visual.EnabledClasses = classes
			}
			if standard != "" {
				cfg.Compliance.Standard = standard
			}
			if auditor != "" {
				cfg.Compliance.AuditorIdentity = auditor
			}

			// Keypair setup for cryptographic compliance signing
			var kp *audit.KeyPair
			if privKey != "" && pubKey != "" {
				loadedKP, err := audit.LoadKeyPair(privKey, pubKey)
				if err != nil {
					return fmt.Errorf("loading keypair: %w", err)
				}
				kp = loadedKP
			} else {
				// Ephemeral keypair generated for this session
				genKP, err := audit.GenerateKeyPair()
				if err != nil {
					return fmt.Errorf("generating keypair: %w", err)
				}
				kp = genKP
			}

			fmt.Printf("Starting redaction on: %s\n", inputPath)
			fmt.Printf("Output: %s | Audit: %s | Compliance: %s\n", outputPath, auditPath, cfg.Compliance.Standard)

			summary, err := media.RunRedactionPipeline(media.PipelineOptions{
				InputPath:  inputPath,
				OutputPath: outputPath,
				AuditPath:  auditPath,
				Config:     cfg,
				KeyPair:    kp,
				ProgressCallback: func(frameIdx, totalFrames int, fps float64) {
					if totalFrames > 0 {
						pct := float64(frameIdx) / float64(totalFrames) * 100.0
						fmt.Printf("\r[Processing] Frame %d/%d (%.1f%%) - Speed: %.1f FPS", frameIdx, totalFrames, pct, fps)
					} else {
						fmt.Printf("\r[Processing] Frame %d - Speed: %.1f FPS", frameIdx, fps)
					}
				},
			})
			if err != nil {
				return err
			}

			if pdfPath != "" {
				if err := audit.GenerateCourtCertificatePDF(summary.Manifest, pdfPath); err == nil {
					fmt.Printf("Courtroom PDF certificate generated: %s\n", pdfPath)
				}
			}

			fmt.Println("\nRedaction complete.")
			fmt.Printf("Frames processed: %d (%.2f seconds footage)\n", summary.TotalFrames, summary.DurationSec)
			fmt.Printf("Elapsed: %.2fs (Average throughput: %.1f FPS)\n", summary.ProcessingTimeSec, summary.EffectiveFPS)
			fmt.Printf("Visual targets redacted: %d\n", summary.VisualDetections)
			fmt.Printf("Audio PII bleeps applied: %d\n", summary.AudioPIIBleeps)
			fmt.Printf("Cryptographic root hash: %s\n", summary.Manifest.FrameHashRoot)
			fmt.Printf("Compliance audit saved to: %s\n", auditPath)

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputPath, "input", "i", "", "Input video file path (required)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "redacted.mp4", "Output redacted video file path")
	cmd.Flags().StringVarP(&auditPath, "audit", "a", "redaction_audit.json", "Cryptographic audit certificate file path")
	cmd.Flags().StringVarP(&mode, "mode", "m", "gaussian", "Redaction mode: gaussian, mosaic, blackout")
	cmd.Flags().Float64Var(&sigma, "sigma", 18.0, "Gaussian blur sigma strength")
	cmd.Flags().IntVar(&blockSize, "block-size", 24, "Mosaic pixelation block size")
	cmd.Flags().StringVar(&targets, "targets", "face,license_plate,computer_screen,id_badge", "Comma-separated target classes")
	cmd.Flags().StringVar(&standard, "standard", "CJIS", "Regulatory standard: CJIS, HIPAA, FOIA, GDPR")
	cmd.Flags().StringVar(&auditor, "auditor", "videoblur-autonomous-agent", "Auditor identity name")
	cmd.Flags().StringVar(&privKey, "privkey", "", "Optional private key path for signing")
	cmd.Flags().StringVar(&pubKey, "pubkey", "", "Optional public key path for signing")

	cmd.Flags().StringVar(&pdfPath, "pdf", "", "Optional path to export a formal court PDF certificate")

	return cmd
}

func newScanCmd() *cobra.Command {
	var inputPath string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Perform dry-run detection scan on video and audio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return fmt.Errorf("input video path required (--input)")
			}

			info, err := media.ProbeMedia(inputPath)
			if err != nil {
				return err
			}

			detector, err := vision.NewVisualDetector("")
			if err != nil {
				return err
			}
			defer detector.Close()

			demuxer, err := media.NewDemuxer(inputPath, info.Width, info.Height)
			if err != nil {
				return err
			}
			defer demuxer.Close()

			fmt.Printf("Forensic Scan: %s (%dx%d, %.2f FPS, %.2fs)\n", inputPath, info.Width, info.Height, info.FPS, info.DurationSec)

			frameBuf := make([]byte, info.Width*info.Height*3)
			frameIdx := 0
			var totalFaces, totalPlates, totalScreens int

			for {
				if err := demuxer.ReadFrame(frameBuf); err != nil {
					break
				}
				dets, _ := detector.Detect(frameBuf, info.Width, info.Height, []config.TargetClass{
					config.TargetFace,
					config.TargetLicensePlate,
					config.TargetComputerScreen,
				}, 0.35)

				for _, d := range dets {
					switch d.Class {
					case config.TargetFace:
						totalFaces++
					case config.TargetLicensePlate:
						totalPlates++
					case config.TargetComputerScreen:
						totalScreens++
					}
				}
				frameIdx++
			}

			fmt.Println("\nScan Results:")
			fmt.Printf("- Scanned Frames: %d\n", frameIdx)
			fmt.Printf("- Faces Detected: %d\n", totalFaces)
			fmt.Printf("- License Plates Detected: %d\n", totalPlates)
			fmt.Printf("- Computer Screens Detected: %d\n", totalScreens)

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputPath, "input", "i", "", "Input video path")

	return cmd
}

func newCertifyCmd() *cobra.Command {
	var (
		manifestPath string
		pdfPath      string
	)

	cmd := &cobra.Command{
		Use:   "certify",
		Short: "Verify a cryptographic compliance audit manifest and signature",
		RunE: func(cmd *cobra.Command, args []string) error {
			if manifestPath == "" {
				manifestPath = "redaction_audit.json"
			}

			res, err := audit.VerifyManifestFile(manifestPath)
			if err != nil {
				return fmt.Errorf("verification error: %w", err)
			}

			out, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(out))

			if pdfPath != "" {
				data, err := os.ReadFile(manifestPath)
				if err == nil {
					var m domain.AuditManifest
					if err := json.Unmarshal(data, &m); err == nil {
						if err := audit.GenerateCourtCertificatePDF(&m, pdfPath); err == nil {
							fmt.Printf("Courtroom PDF certificate generated: %s\n", pdfPath)
						}
					}
				}
			}

			if res.IsValid && res.SignatureValid {
				fmt.Println("\nVERDICT: [VALID] Cryptographic Non-Repudiation Verified.")
				return nil
			}

			fmt.Println("\nVERDICT: [FAILED] Compliance integrity checks failed.")
			os.Exit(1)
			return nil
		},
	}

	cmd.Flags().StringVarP(&manifestPath, "manifest", "m", "redaction_audit.json", "Path to audit manifest file")
	cmd.Flags().StringVar(&pdfPath, "pdf", "", "Optional path to export a formal court PDF certificate")

	return cmd
}

func newStudioCmd() *cobra.Command {
	var (
		port       int
		videoPath  string
		auditPath  string
		sourcePath string
	)

	cmd := &cobra.Command{
		Use:   "studio",
		Short: "Launch interactive embedded review studio",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()
			srv := server.NewServer(port, videoPath, auditPath, sourcePath, cfg)
			return srv.Start()
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8181, "Port to listen on")
	cmd.Flags().StringVarP(&videoPath, "video", "v", "redacted.mp4", "Path to video file to review")
	cmd.Flags().StringVarP(&auditPath, "audit", "a", "redaction_audit.json", "Path to audit manifest file")
	cmd.Flags().StringVarP(&sourcePath, "source", "s", "", "Optional original source video path for re-render overrides")

	return cmd
}

func newWatchCmd() *cobra.Command {
	var (
		inboxDir    string
		outboxDir   string
		archiveDir  string
		concurrency int
		generatePDF bool
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Run automated watchdog daemon for intake folder monitoring",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()

			w, err := watch.NewWatcher(watch.WatcherOptions{
				InboxDir:     inboxDir,
				OutboxDir:    outboxDir,
				ArchiveDir:   archiveDir,
				Concurrency:  concurrency,
				PollInterval: 2 * time.Second,
				Config:       cfg,
				GeneratePDF:  generatePDF,
			})
			if err != nil {
				return err
			}

			// Capture OS signals for graceful shutdown
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return w.Start(ctx)
		},
	}

	cmd.Flags().StringVar(&inboxDir, "inbox", "./inbox", "Folder path to monitor for incoming media files")
	cmd.Flags().StringVar(&outboxDir, "outbox", "./outbox", "Folder path for redacted video, audit manifests, and PDFs")
	cmd.Flags().StringVar(&archiveDir, "archive", "", "Optional folder path to move processed source media")
	cmd.Flags().IntVar(&concurrency, "concurrency", 2, "Number of concurrent worker pipelines")
	cmd.Flags().BoolVar(&generatePDF, "pdf", true, "Generate formal legal PDF certificates")

	return cmd
}
