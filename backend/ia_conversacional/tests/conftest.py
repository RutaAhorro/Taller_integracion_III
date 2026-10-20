import sys
from pathlib import Path

# Asegurar que el directorio del servicio esté en sys.path
servicio_dir = Path(__file__).resolve().parent.parent
if str(servicio_dir) not in sys.path:
    sys.path.insert(0, str(servicio_dir))
