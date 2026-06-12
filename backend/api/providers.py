from fastapi import APIRouter, Depends
from sqlmodel import Session, select
from database.connection import get_session
from database.models import Provider, Model
from pydantic import BaseModel
from typing import Optional
import os
import httpx

router = APIRouter()

class ProviderUpdate(BaseModel):
    api_key: Optional[str] = None
    is_enabled: Optional[bool] = None
    base_url: Optional[str] = None

def _has_key(provider: Provider) -> bool:
    return bool(provider.api_key_value or (provider.api_key_env and os.getenv(provider.api_key_env)))

@router.get("/api/providers")
def list_providers(session: Session = Depends(get_session)):
    providers = session.exec(select(Provider)).all()
    return [
        {
            "name": p.name,
            "display_name": p.display_name,
            "is_enabled": p.is_enabled,
            "is_local": p.is_local,
            "has_key": _has_key(p),
            "base_url": p.base_url,
        }
        for p in providers
    ]

@router.patch("/api/providers/{name}")
def update_provider(name: str, update: ProviderUpdate, session: Session = Depends(get_session)):
    provider = session.exec(select(Provider).where(Provider.name == name)).first()
    if not provider:
        return {"error": "Provider not found"}
    if update.api_key is not None:
        provider.api_key_value = update.api_key
    if update.is_enabled is not None:
        provider.is_enabled = update.is_enabled
    if update.base_url is not None:
        provider.base_url = update.base_url
    session.add(provider)
    session.commit()
    return {"ok": True}

@router.get("/api/models")
def list_models(provider: Optional[str] = None, session: Session = Depends(get_session)):
    providers = {p.name: p for p in session.exec(select(Provider)).all()}
    query = select(Model).where(Model.is_enabled == True)
    if provider:
        query = query.where(Model.provider_name == provider)
    models = session.exec(query).all()
    result = []
    for m in models:
        prov = providers.get(m.provider_name)
        if not prov:
            continue
        if not _has_key(prov) and not prov.is_local:
            continue
        result.append({
            "id": m.model_id,
            "display_name": m.display_name,
            "provider": m.provider_name,
            "provider_display_name": prov.display_name,
            "context_window": m.context_window,
            "supports_reasoning": m.supports_reasoning,
            "supports_vision": m.supports_vision,
            "supports_tools": m.supports_tools,
            "cost_per_1k_input": m.cost_per_1k_input,
            "cost_per_1k_output": m.cost_per_1k_output,
        })
    return result

@router.post("/api/models/sync")
async def sync_models(provider_name: Optional[str] = None, session: Session = Depends(get_session)):
    """Fetch live models from provider APIs and replace stored models.

    If provider_name is omitted, syncs all providers that have API keys set.
    """
    providers_map = {p.name: p for p in session.exec(select(Provider)).all()}
    targets = [provider_name] if provider_name else ["openrouter", "opencode"]
    results = {}

    for name in targets:
        provider = providers_map.get(name)
        if not provider:
            results[name] = "provider not found"
            continue
        if not _has_key(provider):
            results[name] = "no API key"
            continue

        if name == "openrouter":
            api_key = provider.api_key_value or os.getenv(provider.api_key_env or "")
            headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}
            try:
                async with httpx.AsyncClient(timeout=15) as client:
                    resp = await client.get("https://openrouter.ai/api/v1/models", headers=headers)
                    resp.raise_for_status()
                    data = resp.json()
            except Exception as e:
                results[name] = f"fetch failed: {e}"
                continue

            old = session.exec(select(Model).where(Model.provider_name == "openrouter")).all()
            for m in old:
                session.delete(m)

            added = 0
            for item in data.get("data", []):
                mid = f"openrouter/{item['id']}"
                arch = item.get("architecture", {}) or {}
                pricing = item.get("pricing", {}) or {}
                session.add(Model(
                    provider_name="openrouter",
                    model_id=mid,
                    display_name=item.get("name", item["id"]),
                    context_window=item.get("context_length", 8192) or 8192,
                    supports_reasoning=False,
                    supports_vision="image" in str(arch),
                    supports_tools=True,
                    cost_per_1k_input=float(pricing.get("prompt", 0)),
                    cost_per_1k_output=float(pricing.get("completion", 0)),
                ))
                added += 1
            results[name] = f"{added} models synced"

        elif name == "opencode":
            base_url = provider.base_url or "https://opencode.ai/zen/v1"
            api_key = provider.api_key_value or os.getenv(provider.api_key_env or "")
            headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}
            try:
                async with httpx.AsyncClient(timeout=15) as client:
                    resp = await client.get(f"{base_url}/models", headers=headers)
                    resp.raise_for_status()
                    data = resp.json()
            except Exception as e:
                results[name] = f"fetch failed: {e}"
                continue

            old = session.exec(select(Model).where(Model.provider_name == "opencode")).all()
            for m in old:
                session.delete(m)

            items = data if isinstance(data, list) else data.get("data", [])
            added = 0
            for item in items:
                model_id = item.get("id", "")
                if not model_id:
                    continue
                mid = f"opencode/{model_id}"
                pricing = item.get("pricing", item.get("costs", {})) or {}
                session.add(Model(
                    provider_name="opencode",
                    model_id=mid,
                    display_name=item.get("name", model_id),
                    context_window=item.get("context_length", item.get("max_context", 8192)) or 8192,
                    supports_reasoning=item.get("supports_reasoning", False),
                    supports_vision=item.get("supports_vision", "image" in str(item.get("capabilities", {}))),
                    supports_tools=item.get("supports_tools", "tools" in str(item.get("capabilities", {}))),
                    cost_per_1k_input=float(pricing.get("prompt", pricing.get("input", 0))),
                    cost_per_1k_output=float(pricing.get("completion", pricing.get("output", 0))),
                ))
                added += 1
            results[name] = f"{added} models synced"

    session.commit()
    return results
