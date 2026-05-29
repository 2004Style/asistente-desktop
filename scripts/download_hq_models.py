#!/usr/bin/env python3
import os
import sys
import urllib.request
import zipfile
import tempfile
import shutil
import re

# Localización de este script
script_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.dirname(script_dir)
os.chdir(project_root)

def progress(block_num, block_size, total_size):
    read_so_far = block_num * block_size
    if total_size > 0:
        percent = read_so_far * 100.0 / total_size
        s = f"\r   [Progreso] {percent:.1f}% ({read_so_far // (1024*1024)} MB / {total_size // (1024*1024)} MB)"
        sys.stdout.write(s)
        sys.stdout.flush()
    else:
        sys.stdout.write(f"\r   [Progreso] {read_so_far // (1024*1024)} MB descargados")
        sys.stdout.flush()

def update_rbot_yaml(new_model_path):
    paths = [
        "config/rbot.yaml",
        os.path.expanduser("~/.config/rbot/rbot.yaml")
    ]
    updated = False
    for path in paths:
        if not os.path.exists(path):
            continue
        try:
            with open(path, "r", encoding="utf-8") as f:
                content = f.read()
            
            # Buscar whisper_model y cambiar su valor
            new_content = re.sub(
                r"(whisper_model\s*:\s*).*?\n", 
                f"\\g<1>{new_model_path}\n", 
                content
            )
            
            with open(path, "w", encoding="utf-8") as f:
                f.write(new_content)
            print(f"✅ Archivo de configuración '{path}' actualizado con '{new_model_path}'.")
            updated = True
        except Exception as e:
            print(f"⚠️ Error al actualizar '{path}': {e}")
    return updated

def upgrade_vosk():
    dest_dir = "config/vosk_model"
    model_url = "https://alphacephei.com/vosk/models/vosk-model-es-0.42.zip"
    model_zip_name = "vosk-model-es-0.42.zip"
    
    print("\n🚀 [Vosk Upgrade] Descargando modelo profesional en español (1.4 GB)...")
    print("Este proceso puede tomar varios minutos según tu conexión a Internet.")
    print(f"URL: {model_url}")
    
    try:
        with tempfile.TemporaryDirectory() as tmpdir:
            zip_path = os.path.join(tmpdir, model_zip_name)
            urllib.request.urlretrieve(model_url, zip_path, progress)
            print("\n\n📦 Extrayendo modelo de voz pesado...")
            
            with zipfile.ZipFile(zip_path, 'r') as zip_ref:
                zip_ref.extractall(tmpdir)
                
            extracted_folder = os.path.join(tmpdir, "vosk-model-es-0.42")
            if not os.path.exists(extracted_folder):
                print("❌ Error: No se encontró la carpeta extraída esperada 'vosk-model-es-0.42'.")
                return False
                
            # Mover la carpeta extraída a la ruta de configuración final
            if os.path.exists(dest_dir):
                print("🗑️ Limpiando modelo antiguo...")
                shutil.rmtree(dest_dir)
            
            os.makedirs(os.path.dirname(dest_dir), exist_ok=True)
            shutil.move(extracted_folder, dest_dir)
            
        print(f"🎉 ¡Modelo profesional de Vosk instalado con éxito en '{dest_dir}'!")
        return True
    except Exception as e:
        print(f"\n❌ Error al actualizar el modelo de Vosk: {e}")
        return False

def upgrade_whisper():
    print("\nSelecciona el tamaño del modelo Whisper (GGML) que deseas descargar:")
    print(" [1] Base (~140 MB) - Rápido, precisión aceptable en español.")
    print(" [2] Small (~460 MB) - (Recomendado) Excelente precisión, buen rendimiento en CPU/GPU.")
    print(" [3] Medium (~1.5 GB) - Máxima precisión, mayor consumo de recursos.")
    
    choice = input("\nElige una opción (1-3): ").strip()
    
    model_name = "small"
    if choice == "1":
        model_name = "base"
    elif choice == "2":
        model_name = "small"
    elif choice == "3":
        model_name = "medium"
    else:
        print("Opción inválida. Cancelando descarga de Whisper.")
        return False
        
    model_filename = f"ggml-{model_name}.bin"
    model_url = f"https://huggingface.co/ggerganov/whisper.cpp/resolve/main/{model_filename}"
    
    dest_dir = "models"
    os.makedirs(dest_dir, exist_ok=True)
    dest_path = os.path.join(dest_dir, model_filename)
    
    print(f"\n🚀 [Whisper Upgrade] Descargando modelo Whisper '{model_name}' ({model_filename})...")
    print(f"URL: {model_url}")
    
    try:
        urllib.request.urlretrieve(model_url, dest_path, progress)
        print(f"\n\n🎉 Modelo Whisper '{model_name}' descargado con éxito en '{dest_path}'.")
        
        # Actualizar rbot.yaml
        update_rbot_yaml(dest_path)
        return True
    except Exception as e:
        print(f"\n❌ Error al descargar el modelo de Whisper: {e}")
        return False

def main():
    print("=========================================================")
    print("   RBot - Actualización de Modelos de Voz de Alta Calidad")
    print("=========================================================")
    print("¿Qué modelo deseas actualizar para mejorar la transcripción?")
    print(" [1] Vosk (Transcriptor en tiempo real del Micrófono)")
    print("     -> Descarga el modelo profesional de 1.4 GB (elimina eco, entiende jerga).")
    print(" [2] Whisper (Procesador offline del Daemon)")
    print("     -> Permite descargar modelos Base/Small/Medium (más precisos que ggml-tiny).")
    print(" [3] Ambos modelos (Vosk Profesional + Whisper HQ)")
    
    opt = input("\nElige una opción (1-3): ").strip()
    
    if opt == "1":
        upgrade_vosk()
    elif opt == "2":
        upgrade_whisper()
    elif opt == "3":
        v_ok = upgrade_vosk()
        w_ok = upgrade_whisper()
        if v_ok and w_ok:
            print("\n🎉 ¡Todos los modelos se actualizaron con éxito!")
    else:
        print("Opción inválida. Saliendo.")

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n👋 Descarga cancelada por el usuario.")
        sys.exit(0)
