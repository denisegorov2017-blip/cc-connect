"""Transport tests with only the external Hermes recognizer mocked."""
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest


class HermesCommandTransportTests(unittest.TestCase):
    def test_recognition_contract_and_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            tools = root / "tools"
            tools.mkdir()
            (tools / "__init__.py").write_text("", encoding="utf-8")
            (tools / "transcription_tools.py").write_text(
                "import os, pathlib\n"
                "def transcribe_audio_local_fallback(path, model):\n"
                "    assert os.environ['HF_HUB_OFFLINE']=='1'\n"
                "    assert pathlib.Path(path).read_bytes()==b'unit-audio'\n"
                "    assert pathlib.Path(model,'model.bin').is_file()\n"
                "    return {'success':True,'transcript':'Тест голоса'}\n", encoding="utf-8")
            (root / "model.bin").write_bytes(b"unit-model-fixture")
            completed = subprocess.run(
                [sys.executable, str(pathlib.Path(__file__).with_name("hermes_local_stt.py")),
                 "--hermes-dir", str(root), "--model-dir", str(root), "--evidence-dir", str(root / "evidence")],
                input=b"unit-audio", capture_output=True, timeout=10,
                env={**os.environ, "CC_CONNECT_STT_FORMAT": "mp3"}, check=True)
            self.assertEqual(json.loads(completed.stdout), {"text": "Тест голоса"})
            self.assertEqual(len(list((root / "evidence").glob("*.mp3"))), 1)
            report = json.loads(next((root / "evidence").glob("*.json")).read_text(encoding="utf-8"))
            self.assertEqual(report["audio_bytes"], len(b"unit-audio"))

    def test_missing_model_fails_without_provider_output(self):
        with tempfile.TemporaryDirectory() as directory:
            completed = subprocess.run(
                [sys.executable, str(pathlib.Path(__file__).with_name("hermes_local_stt.py")),
                 "--hermes-dir", directory, "--model-dir", directory],
                input=b"unit-audio", capture_output=True, timeout=10,
                env={**os.environ, "CC_CONNECT_STT_FORMAT": "mp3"}, check=False)
            self.assertNotEqual(completed.returncode, 0)
            self.assertEqual(completed.stdout, b"")
            self.assertNotIn(directory.encode(), completed.stderr)


if __name__ == "__main__":
    unittest.main()
