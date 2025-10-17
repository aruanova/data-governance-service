# Modular Refinery Architecture Proposal

## Overview

Transform the current language-based refinery system into a version-based, plugin architecture that follows SOLID principles and enables easy extension.

## Core Design Principles

1. **Open/Closed Principle**: Open for extension, closed for modification
2. **Single Responsibility**: Each refinery handles one cleaning strategy
3. **Dependency Inversion**: Depend on abstractions, not concrete implementations
4. **Interface Segregation**: Small, focused interfaces
5. **Liskov Substitution**: All refineries are interchangeable

## Architecture Components

### 1. Base Refinery Interface
```python
# backend/app/refinery/base.py
from abc import ABC, abstractmethod
from typing import List, Dict, Any, Callable, Optional

class BaseRefinery(ABC):
    """Base interface for all text refineries"""
    
    @abstractmethod
    def __init__(self, config: Optional[Dict[str, Any]] = None):
        """Initialize refinery with optional config"""
        pass
    
    @abstractmethod
    def process(self, text: str) -> str:
        """Process a single text string"""
        pass
    
    @abstractmethod
    def get_version(self) -> str:
        """Return the version identifier of this refinery"""
        pass
    
    @abstractmethod
    def get_description(self) -> str:
        """Return a human-readable description"""
        pass
    
    @abstractmethod
    def get_default_config(self) -> Dict[str, Any]:
        """Return default configuration"""
        pass
```

### 2. Refinery Registry
```python
# backend/app/refinery/registry.py
from typing import Dict, Type, Optional
from .base import BaseRefinery

class RefineryRegistry:
    """Registry for managing available refineries"""
    
    _refineries: Dict[str, Type[BaseRefinery]] = {}
    _aliases: Dict[str, str] = {}  # For backward compatibility
    
    @classmethod
    def register(cls, version: str, refinery_class: Type[BaseRefinery], aliases: Optional[List[str]] = None):
        """Register a refinery with version and optional aliases"""
        cls._refineries[version] = refinery_class
        
        if aliases:
            for alias in aliases:
                cls._aliases[alias] = version
    
    @classmethod
    def get(cls, identifier: str) -> Type[BaseRefinery]:
        """Get refinery by version or alias"""
        # Check if it's an alias first (for backward compatibility)
        if identifier in cls._aliases:
            identifier = cls._aliases[identifier]
        
        if identifier not in cls._refineries:
            raise ValueError(f"Refinery '{identifier}' not found. Available: {list(cls._refineries.keys())}")
        
        return cls._refineries[identifier]
    
    @classmethod
    def list_available(cls) -> Dict[str, Dict[str, str]]:
        """List all available refineries with metadata"""
        result = {}
        for version, refinery_class in cls._refineries.items():
            instance = refinery_class()
            result[version] = {
                "description": instance.get_description(),
                "aliases": [alias for alias, v in cls._aliases.items() if v == version]
            }
        return result
```

### 3. Updated Pipeline
```python
# backend/app/refinery/pipeline.py
import pandas as pd
import logging
from typing import List, Any, Dict, Optional
from .registry import RefineryRegistry
from .base import BaseRefinery

logger = logging.getLogger(__name__)

class RefineryPipeline:
    def __init__(self, refinery_version: str = "v1-standard", custom_config: Optional[Dict[str, Any]] = None):
        """
        Initialize pipeline with a specific refinery version
        
        Args:
            refinery_version: Version identifier or alias (e.g., "v1-standard", "v2-aggressive", "spanish", "english")
            custom_config: Optional custom configuration to override defaults
        """
        refinery_class = RefineryRegistry.get(refinery_version)
        self.refinery: BaseRefinery = refinery_class(custom_config)
        self.refinery_version = self.refinery.get_version()
        
    def clean_df(self, df: pd.DataFrame, columns: List[str]) -> pd.DataFrame:
        """Clean specified columns in DataFrame"""
        cleaned_df = df.copy()
        for col in columns:
            if col in cleaned_df.columns:
                cleaned_df[f"Clean{col}"] = cleaned_df[col].apply(
                    lambda x: self.refinery.process(str(x)) if pd.notna(x) else x
                )
        return cleaned_df
    
    # ... rest of the existing methods remain the same ...
```

### 4. Concrete Refineries (Version-based)
```python
# backend/app/refinery/versions/v1_standard.py
from typing import List, Dict, Any, Callable, Optional
from ..base import BaseRefinery
from ..processing_nodes import ProcessingNodes

class StandardRefineryV1(BaseRefinery):
    """Standard text cleaning for Mexican business data"""
    
    def __init__(self, config: Optional[Dict[str, Any]] = None):
        self.config = self.get_default_config()
        if config:
            self.config.update(config)
        
        self.nodes = ProcessingNodes(self.config)
        self.pipeline = self._build_pipeline()
    
    def get_version(self) -> str:
        return "v1-standard"
    
    def get_description(self) -> str:
        return "Standard cleaning with Spanish month removal and special character handling"
    
    def get_default_config(self) -> Dict[str, Any]:
        return {
            "allowed_chars": "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyzÁÉÍÓÚáéíóúÑñ0123456789 ",
            "to_keep": {"SI", "NO", "GPS", "MPLS", "DSL", "MXN", "MXP", "USD", "RX", "TC", "TG"},
            "to_remove": {
                "ENERO", "FEBRERO", "MARZO", "ABRIL", "MAYO", "JUNIO", "JULIO", "AGOSTO",
                "SEPTIEMBRE", "OCTUBRE", "NOVIEMBRE", "DICIEMBRE",
                "ENE", "FEB", "MAR", "ABR", "MAY", "JUN", "JUL", "AGO", "SEP", "OCT", "NOV", "DIC",
                "DE", "DEL"
            },
            "min_len": 3,
            "sep_chars": ".,-/+&|",
            "make_lowercase": True,
            # ... other config options ...
        }
    
    def _build_pipeline(self) -> List[Callable[[str], str]]:
        return [
            self.nodes.remove_prefixed_codes,
            self.nodes.remove_trailing_solicitante,
            self.nodes.replace_separators,
            self.nodes.remove_multiple_whitespace,
            self.nodes.remove_special_chars,
            self.nodes.remove_words_from_list,
            self.nodes.remove_alphanumeric_words,
            self.nodes.remove_all_numbers_words_except,
            self.nodes.remove_words_by_min_len,
            self.nodes.remove_all_consonants_words,
            self.nodes.make_lowercase
        ]
    
    def process(self, text: str) -> str:
        if not isinstance(text, str):
            text = str(text)
        for step in self.pipeline:
            text = step(text)
        return text
```

### 5. Processing Nodes (Shared Logic)
```python
# backend/app/refinery/processing_nodes.py
import re
import unicodedata
from typing import Dict, Any

class ProcessingNodes:
    """Reusable processing nodes for text refineries"""
    
    def __init__(self, config: Dict[str, Any]):
        self.config = config
    
    def normalize_nfkd(self, text: str) -> str:
        if self.config.get("normalize_nfkd", False):
            return "".join(
                c for c in unicodedata.normalize('NFKD', text) 
                if not unicodedata.combining(c)
            )
        return text
    
    def make_uppercase(self, text: str) -> str:
        return text.upper() if self.config.get("make_uppercase", False) else text
    
    def make_lowercase(self, text: str) -> str:
        return text.lower() if self.config.get("make_lowercase", False) else text
    
    # ... implement all other processing methods ...
```

### 6. Auto-registration System
```python
# backend/app/refinery/__init__.py
from .registry import RefineryRegistry
from .pipeline import RefineryPipeline
from .versions import v1_standard, v2_aggressive, v3_minimal

# Auto-register all refineries
RefineryRegistry.register(
    "v1-standard", 
    v1_standard.StandardRefineryV1,
    aliases=["spanish", "standard"]  # Backward compatibility
)

RefineryRegistry.register(
    "v2-aggressive",
    v2_aggressive.AggressiveRefineryV2,
    aliases=["english", "aggressive"]  # Backward compatibility
)

RefineryRegistry.register(
    "v3-minimal",
    v3_minimal.MinimalRefineryV3,
    aliases=["minimal", "basic"]
)

__all__ = ["RefineryPipeline", "RefineryRegistry"]
```

## Migration Strategy

### Phase 1: Add New Architecture (Backward Compatible)
1. Implement base classes and registry
2. Create version-based refineries that wrap existing ones
3. Update pipeline to use registry while maintaining old API

### Phase 2: Gradual Migration
1. Update API to accept `refinery_version` parameter
2. Add deprecation warnings for `refinery_type`
3. Update frontend to use new parameter

### Phase 3: Cleanup
1. Remove old refinery classes
2. Remove backward compatibility aliases
3. Update documentation

## Benefits

1. **Easy Extension**: Just create a new class implementing `BaseRefinery` and register it
2. **Version Control**: Clear versioning allows tracking changes over time
3. **Backward Compatibility**: Aliases ensure existing code continues to work
4. **Configuration Flexibility**: Each refinery can have its own config schema
5. **Testability**: Each refinery can be tested independently
6. **Documentation**: Self-documenting through `get_description()` method

## Example: Adding a New Refinery

```python
# backend/app/refinery/versions/v4_custom.py
from ..base import BaseRefinery

class CustomRefineryV4(BaseRefinery):
    def __init__(self, config=None):
        # Implementation
    
    def get_version(self):
        return "v4-custom"
    
    def get_description(self):
        return "Custom refinery for specific use case"
    
    # ... implement other methods ...

# In __init__.py
RefineryRegistry.register("v4-custom", CustomRefineryV4)
```

## API Updates

### Current API
```python
request = CleaningSampleRequest(
    upload_id="123",
    refinery_type="spanish",  # Hard-coded language
    columns_to_clean=["description"]
)
```

### New API (with backward compatibility)
```python
request = CleaningSampleRequest(
    upload_id="123",
    refinery_version="v2-aggressive",  # Version-based
    columns_to_clean=["description"]
)
```

## Configuration Management

Each refinery can expose its configuration schema:

```python
@abstractmethod
def get_config_schema(self) -> Dict[str, Any]:
    """Return JSON schema for configuration validation"""
    return {
        "type": "object",
        "properties": {
            "allowed_chars": {"type": "string"},
            "min_len": {"type": "integer", "minimum": 1},
            # ... other properties ...
        }
    }
```

This allows for:
- Dynamic UI generation for configuration
- Runtime validation
- Better documentation