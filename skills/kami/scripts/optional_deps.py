"""Centralized loader for optional third-party deps (weasyprint, pypdf, PyMuPDF).

build.py previously had inline `from weasyprint import HTML` try/except blocks
with duplicated install hints; this module collapses those into one resolver
each so the import error message stays consistent and the WeasyPrint runtime
is configured once at the import call site.
"""
from __future__ import annotations

import sys

from shared import configure_weasyprint_runtime

# On Linux, WeasyPrint links against cairo / pango / harfbuzz at runtime; a bare
# `pip install weasyprint` succeeds but then fails to load with a cryptic
# "cannot load library 'libgobject-2.0'" until the native libs are present. Spell
# them out so the error message is actionable on a fresh Linux box.
_LINUX_NATIVE_LIBS = (
    "Linux also needs native libs: "
    "sudo apt-get install -y libcairo2 libpango-1.0-0 libpangoft2-1.0-0 libharfbuzz0b "
    "(Debian/Ubuntu), or `sudo dnf install cairo pango harfbuzz` (Fedora/RHEL)"
)

WEASYPRINT_INSTALL_HINT = "pip install weasyprint pypdf --break-system-packages"
if sys.platform.startswith("linux"):
    WEASYPRINT_INSTALL_HINT = f"{WEASYPRINT_INSTALL_HINT}. {_LINUX_NATIVE_LIBS}"
PYMUPDF_INSTALL_HINT = "pip install pymupdf --break-system-packages"


class MissingDepError(RuntimeError):
    """Raised when an optional dependency is requested but not installed."""


def require_weasyprint_html():
    """Return the weasyprint.HTML class, configuring native libs first."""
    configure_weasyprint_runtime()
    try:
        from weasyprint import HTML
        return HTML
    except ImportError as exc:
        raise MissingDepError(
            f"missing weasyprint. {WEASYPRINT_INSTALL_HINT}"
        ) from exc


def _require_pypdf_attr(name: str):
    try:
        import pypdf
    except ImportError as exc:
        raise MissingDepError(
            f"missing pypdf. {WEASYPRINT_INSTALL_HINT}"
        ) from exc
    return getattr(pypdf, name)


def require_pypdf_reader():
    """Return the pypdf.PdfReader class."""
    return _require_pypdf_attr("PdfReader")


def require_pypdf_writer():
    """Return the pypdf.PdfWriter class."""
    return _require_pypdf_attr("PdfWriter")


def require_pymupdf():
    """Return the PyMuPDF module (imported as fitz)."""
    try:
        import fitz
        return fitz
    except ImportError as exc:
        raise MissingDepError(
            f"missing PyMuPDF. {PYMUPDF_INSTALL_HINT}"
        ) from exc
