#!/usr/bin/env python3
import os
import sys
import urllib.request
import zipfile
import tempfile
import shutil

def download_and_extract_model():
    dest_dir = "config/vosk_model"
    
    # Si ya existe y no está vacío, no hacer nada
    if os.path.exists(dest_dir) and os.listdir(dest_dir):
        print(f"✅ El modelo de Vosk ya está instalado en '{dest_dir}'.")
        return True

    os.makedirs("config", exist_ok=True)
    
    model_url = "https://alphacephei.com/vosk/models/vosk-model-small-es-0.42.zip"
    model_zip_name = "vosk-model-small-es-0.42.zip"
    
    print("🎙️ [Vosk Setup] Descargando modelo de voz en español (20MB aprox)...")
    print(f"URL: {model_url}")
    
    try:
        # Descargar a un directorio temporal
        with tempfile.TemporaryDirectory() as tmpdir:
            zip_path = os.path.join(tmpdir, model_zip_name)
            
            # Descarga con progreso simple
            def progress(block_num, block_size, total_size):
                read_so_far = block_num * block_size
                if total_size > 0:
                    percent = read_so_far * 1e2 / total_size
                    s = f"\r   Progreso: {percent:.1f}% ({read_so_far // 1024} KB / {total_size // 1024} KB)"
                    sys.stdout.write(s)
                    sys.stdout.flush()
                else:
                    sys.stdout.write(f"\r   Progreso: {read_so_far // 1024} KB descargados")
                    sys.stdout.flush()

            urllib.request.urlretrieve(model_url, zip_path, progress)
            print("\n\n📦 Extrayendo modelo de voz...")
            
            with zipfile.ZipFile(zip_path, 'r') as zip_ref:
                zip_ref.extractall(tmpdir)
                
            extracted_folder = os.path.join(tmpdir, "vosk-model-small-es-0.42")
            if not os.path.exists(extracted_folder):
                print("❌ Error: No se encontró la carpeta extraída esperada.")
                return False
                
            # Mover la carpeta extraída a la ruta destino final
            if os.path.exists(dest_dir):
                shutil.rmtree(dest_dir)
            shutil.move(extracted_folder, dest_dir)
            
        print(f"🎉 Modelo de Vosk instalado con éxito en '{dest_dir}'.")
        return True
    except Exception as e:
        print(f"\n❌ Error al descargar o instalar el modelo de Vosk: {e}")
        return False

if __name__ == "__main__":
    success = download_and_extract_model()
    sys.exit(0 if success else 1)
