"""Run one CaptionGenerator job locally or as a Stash raw-interface task."""

from __future__ import annotations

import argparse
import asyncio
import json
import os
from pathlib import Path
import shutil
import sys
from typing import Any


_STASH_MODE = False


def stash_log(level: str, message: Any) -> None:
    print(f"\x01{level}\x02 {message}", file=sys.stderr, flush=True)


def emit(event_type: str, **values: Any) -> None:
    if _STASH_MODE:
        if event_type == "progress":
            progress = max(0.0, min(100.0, float(values.get("progress", 0))))
            stash_log("p", progress / 100.0)
            stage = str(values.get("stage", "") or "").strip()
            if stage:
                stash_log("d", f"{progress:.1f}% · {stage}")
        elif event_type == "log":
            message = str(values.get("message", "") or "").strip()
            if message:
                stash_log("i", message)
        return
    payload = {"type": event_type}
    payload.update(values)
    print(json.dumps(payload, ensure_ascii=False), flush=True)


class LabelEmitter:
    def __init__(self, progress: "ProgressEmitter") -> None:
        self.progress = progress

    def set_description_str(self, value: str) -> None:
        stage = str(value or "").strip()
        if " - " in stage:
            stage = stage.rsplit(" - ", 1)[-1]
        self.progress.stage = stage or self.progress.stage
        self.progress.publish(force=True)

    def refresh(self) -> None:
        self.progress.publish()


class ProgressEmitter:
    total = 100
    worker_idx = 1

    def __init__(self, video_name: str) -> None:
        self.video_name = video_name
        self.stage = "Starting"
        self._n = 1.0
        self._last_progress = -1.0
        self._last_stage = ""
        self.label_bar = LabelEmitter(self)

    @property
    def n(self) -> float:
        return self._n

    @n.setter
    def n(self, value: float) -> None:
        try:
            self._n = max(0.0, min(100.0, float(value)))
        except (TypeError, ValueError):
            return
        self.publish()

    def publish(self, force: bool = False) -> None:
        if (
            not force
            and abs(self._n - self._last_progress) < 0.5
            and self.stage == self._last_stage
        ):
            return
        self._last_progress = self._n
        self._last_stage = self.stage
        emit("progress", progress=round(self._n, 1), stage=self.stage)

    def set_description(self, value: str) -> None:
        self.stage = str(value or "").strip() or self.stage
        self.publish(force=True)

    def set_description_str(self, value: str) -> None:
        self.set_description(value)

    def set_postfix_str(self, value: str) -> None:
        value = str(value or "").strip()
        if value and not value.startswith("Processing "):
            self.stage = value
            self.publish(force=True)

    def refresh(self) -> None:
        self.publish()

    def reset(self) -> None:
        self.n = 0

    def write(self, value: str) -> None:
        emit("log", message=str(value or "").strip())


def parse_cli_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--generator-path", required=True)
    parser.add_argument("--video", required=True)
    parser.add_argument("--env-file", default="")
    parser.add_argument("--source-language", choices=("ja", "zh"), required=True)
    parser.add_argument(
        "--mode",
        choices=("transcribe_translate", "transcribe_only", "translate_only"),
        required=True,
    )
    parser.add_argument("--transcription-service", required=True)
    parser.add_argument("--transcription-model", required=True)
    parser.add_argument("--translation-service", required=True)
    parser.add_argument("--translation-model", required=True)
    return parser.parse_args()


def parse_stash_args() -> argparse.Namespace:
    payload = json.load(sys.stdin)
    configure_stash_environment(payload)
    values = payload.get("args")
    if not isinstance(values, dict):
        raise RuntimeError("Stash did not provide subtitle task arguments")

    def required(name: str) -> str:
        value = str(values.get(name, "") or "").strip()
        if not value:
            raise RuntimeError(f"subtitle task argument {name} is missing")
        return value

    return argparse.Namespace(
        generator_path=required("generatorPath"),
        video=required("video"),
        env_file=str(values.get("envFile", "") or "").strip(),
        source_language=required("sourceLanguage"),
        mode=required("mode"),
        transcription_service=required("transcriptionService"),
        transcription_model=required("transcriptionModel"),
        translation_service=required("translationService"),
        translation_model=required("translationModel"),
    )


def configure_stash_environment(payload: dict[str, Any]) -> None:
    connection = payload.get("server_connection")
    if not isinstance(connection, dict):
        return
    stash_directory = str(connection.get("Dir", "") or "").strip()
    if not stash_directory or not Path(stash_directory).is_dir():
        return

    current_path = os.environ.get("PATH", "")
    path_entries = current_path.split(os.pathsep) if current_path else []
    normalized = {os.path.normcase(os.path.abspath(entry)) for entry in path_entries if entry}
    normalized_stash = os.path.normcase(os.path.abspath(stash_directory))
    if normalized_stash not in normalized:
        os.environ["PATH"] = stash_directory + (os.pathsep + current_path if current_path else "")

    ffmpeg = shutil.which("ffmpeg")
    if ffmpeg:
        stash_log("i", f"Using FFmpeg: {ffmpeg}")


def load_generator(args: argparse.Namespace):
    generator_path = Path(args.generator_path).resolve()
    script = generator_path / "video_caption_generator.py"
    if not script.is_file():
        raise RuntimeError("video_caption_generator.py was not found")
    sys.path.insert(0, str(generator_path))
    import video_caption_generator as generator  # type: ignore

    if args.env_file and generator.load_dotenv:
        generator.load_dotenv(dotenv_path=Path(args.env_file), override=False)
    return generator


def validate_runtime(generator, args: argparse.Namespace) -> None:
    if args.mode != "translate_only":
        availability = {
            "gemini": generator.GEMINI_AVAILABLE,
            "assemblyai": generator.ASSEMBLYAI_AVAILABLE,
            "granite": generator.GRANITE_AVAILABLE,
            "qwen3": generator.QWEN3_ASR_AVAILABLE,
            "kotoba": generator.KOTOBA_AVAILABLE,
            "reazonspeech": generator.REAZONSPEECH_AVAILABLE,
        }
        if args.transcription_service in availability and not availability[args.transcription_service]:
            raise RuntimeError(
                f"{args.transcription_service} transcription dependencies are not installed"
            )
        if args.source_language != "ja" and args.transcription_service in (
            "granite",
            "kotoba",
            "reazonspeech",
        ):
            raise RuntimeError("the selected transcription service supports Japanese only")

    if args.mode != "transcribe_only":
        if args.translation_service == "gemini" and not generator.GEMINI_AVAILABLE:
            raise RuntimeError("Gemini dependencies are not installed")
        if args.translation_service == "local_llm":
            available, message = generator._check_ollama_available(args.translation_model)
            if not available:
                raise RuntimeError(message)

    needs_openai = (
        args.mode != "translate_only" and args.transcription_service == "openai"
    ) or (
        args.mode != "transcribe_only" and args.translation_service == "openai"
    )
    needs_gemini = (
        args.mode != "translate_only" and args.transcription_service == "gemini"
    ) or (
        args.mode != "transcribe_only" and args.translation_service == "gemini"
    )
    needs_assemblyai = (
        args.mode != "translate_only" and args.transcription_service == "assemblyai"
    )
    if needs_openai and not os.getenv("OPENAI_API_KEY"):
        raise RuntimeError("OPENAI_API_KEY is missing")
    if needs_gemini and not os.getenv("GOOGLE_API_KEY"):
        raise RuntimeError("GOOGLE_API_KEY is missing")
    if needs_assemblyai and not os.getenv("ASSEMBLYAI_API_KEY"):
        raise RuntimeError("ASSEMBLYAI_API_KEY is missing")


async def build_clients(generator, args: argparse.Namespace):
    openai_client = None
    needs_openai = (
        args.mode != "translate_only" and args.transcription_service == "openai"
    ) or (
        args.mode != "transcribe_only" and args.translation_service == "openai"
    )
    if needs_openai:
        import httpx

        openai_client = generator.openai.AsyncOpenAI(
            api_key=os.getenv("OPENAI_API_KEY"),
            max_retries=3,
            timeout=httpx.Timeout(300.0, connect=60.0),
            http_client=httpx.AsyncClient(
                limits=httpx.Limits(
                    max_keepalive_connections=20,
                    max_connections=50,
                    keepalive_expiry=30.0,
                )
            ),
        )

    needs_gemini = (
        args.mode != "translate_only" and args.transcription_service == "gemini"
    ) or (
        args.mode != "transcribe_only" and args.translation_service == "gemini"
    )
    if needs_gemini:
        generator.gemini_client = generator.genai.Client(
            api_key=os.getenv("GOOGLE_API_KEY")
        )
    if args.mode != "translate_only" and args.transcription_service == "assemblyai":
        generator.aai.settings.api_key = os.getenv("ASSEMBLYAI_API_KEY")
    return openai_client


async def run_job(generator, args: argparse.Namespace) -> None:
    video_path = Path(args.video).resolve()
    if not video_path.is_file():
        raise RuntimeError("the selected video file is unavailable")

    original_srt = video_path.with_name(f"{video_path.stem}.jp.srt")
    translated_srt = video_path.with_suffix(".srt")
    if args.mode in ("transcribe_translate", "translate_only") and translated_srt.exists():
        raise RuntimeError(
            f"{translated_srt.name} already exists; delete it before generating a replacement"
        )

    progress = ProgressEmitter(video_path.name)
    progress.publish(force=True)
    openai_client = await build_clients(generator, args)
    quota = {"exceeded": False}
    try:
        segments = None
        if original_srt.exists():
            progress.stage = f"Loading {original_srt.name}"
            progress.n = 68
            segments = generator.parse_srt_file(original_srt)
            if not segments:
                if args.mode == "translate_only":
                    raise RuntimeError(f"{original_srt.name} is empty or invalid")
                segments = None

        if segments is None:
            if args.mode == "translate_only":
                raise RuntimeError(
                    f"{original_srt.name} is required for translate-only mode"
                )
            progress.stage = "Transcribing"
            progress.n = 5
            segments = await generator.transcribe_video(
                video_path,
                args.transcription_service,
                openai_client,
                args.transcription_model,
                args.source_language,
                progress,
            )
            segments = generator.clean_transcription_segments(segments)
            if not segments:
                raise RuntimeError("transcription produced no subtitle segments")
            progress.stage = f"Saving {original_srt.name}"
            progress.n = 70
            original_content = await generator.generate_srt_content(
                segments=segments,
                translation_service=args.translation_service,
                openai_client=openai_client,
                translate=False,
                gemini_model=args.translation_model,
                openai_model=args.translation_model,
                source_language=args.source_language,
                progress_callback=None,
                quota_exceeded_flag=quota,
            )
            original_srt.write_text(original_content, encoding="utf-8")

        if args.mode == "transcribe_only":
            progress.stage = f"Saved {original_srt.name}"
            progress.n = 100
            return

        progress.stage = "Translating to Traditional Chinese"
        progress.n = 75

        def update_translation(completed: int, total: int) -> None:
            if completed < 0:
                progress.stage = "Translation failed"
            else:
                progress.stage = f"Translating {completed}/{total} segments"
                if total > 0:
                    progress.n = 75 + (completed / total) * 15
            progress.publish(force=True)

        translated_content = await generator.generate_srt_content(
            segments=segments,
            translation_service=args.translation_service,
            openai_client=openai_client,
            translate=True,
            gemini_model=args.translation_model,
            openai_model=args.translation_model,
            source_language=args.source_language,
            progress_callback=update_translation,
            quota_exceeded_flag=quota,
        )
        if quota.get("exceeded"):
            raise RuntimeError("API quota exceeded")
        progress.stage = f"Saving {translated_srt.name}"
        progress.n = 92
        generator.save_subtitle_file(video_path, translated_content, progress)
        progress.stage = f"Saved {translated_srt.name}"
        progress.n = 100
    finally:
        if openai_client:
            await openai_client.close()


def main() -> int:
    global _STASH_MODE
    _STASH_MODE = "--stash-plugin" in sys.argv[1:]
    try:
        args = parse_stash_args() if _STASH_MODE else parse_cli_args()
        generator = load_generator(args)
        validate_runtime(generator, args)
        asyncio.run(run_job(generator, args))
        emit("progress", progress=100, stage="Subtitle generation complete")
        if _STASH_MODE:
            print(
                json.dumps(
                    {"output": f"Generated subtitles for {Path(args.video).name}"},
                    ensure_ascii=False,
                ),
                flush=True,
            )
        return 0
    except Exception as exc:
        emit("progress", progress=-1, stage="Subtitle generation failed")
        if _STASH_MODE:
            stash_log("e", str(exc))
            print(json.dumps({"error": str(exc)}, ensure_ascii=False), flush=True)
            return 1
        print(str(exc), file=sys.stderr, flush=True)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
