from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager
from database.connection import create_tables, engine
from database.models import Provider, Model
from sqlmodel import Session, select
from api.chat import router as chat_router
from api.providers import router as providers_router
from api.conversations import router as conversations_router
from api.settings import router as settings_router
import os
import httpx
from dotenv import load_dotenv

load_dotenv()


def _has_key(provider: Provider) -> bool:
    return bool(provider.api_key_value or (provider.api_key_env and os.getenv(provider.api_key_env)))


def seed_providers():
    with Session(engine) as session:
        existing_providers = {p.name: p for p in session.exec(select(Provider)).all()}

        providers = [
            Provider(name="openrouter", display_name="OpenRouter",
                     api_key_env="OPENROUTER_API_KEY"),
            Provider(name="openai", display_name="OpenAI",
                     api_key_env="OPENAI_API_KEY"),
            Provider(name="anthropic", display_name="Anthropic",
                     api_key_env="ANTHROPIC_API_KEY"),
            Provider(name="groq", display_name="Groq",
                     api_key_env="GROQ_API_KEY"),
            Provider(name="opencode", display_name="OpenCode",
                     api_key_env="OPENCODE_API_KEY",
                     base_url="https://opencode.ai/zen/v1"),
        ]
        for p in providers:
            if p.name not in existing_providers:
                session.add(p)
        session.commit()


async def sync_openrouter_models(session: Session) -> int:
    provider = session.exec(select(Provider).where(Provider.name == "openrouter")).first()
    if not provider or not _has_key(provider):
        return 0

    api_key = provider.api_key_value or os.getenv(provider.api_key_env or "")
    headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}

    try:
        async with httpx.AsyncClient(timeout=15) as client:
            resp = await client.get("https://openrouter.ai/api/v1/models", headers=headers)
            resp.raise_for_status()
            data = resp.json()
    except Exception as e:
        print(f"[sync] OpenRouter model fetch failed: {e}")
        return 0

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
    return added


async def sync_opencode_models(session: Session) -> int:
    provider = session.exec(select(Provider).where(Provider.name == "opencode")).first()
    if not provider or not _has_key(provider):
        return 0

    base_url = provider.base_url or "https://opencode.ai/zen/v1"
    api_key = provider.api_key_value or os.getenv(provider.api_key_env or "")
    headers = {"Authorization": f"Bearer {api_key}"} if api_key else {}

    try:
        async with httpx.AsyncClient(timeout=15) as client:
            resp = await client.get(f"{base_url}/models", headers=headers)
            resp.raise_for_status()
            data = resp.json()
    except Exception as e:
        print(f"[sync] OpenCode model fetch failed: {e}")
        return 0

    old = session.exec(select(Model).where(Model.provider_name == "opencode")).all()
    for m in old:
        session.delete(m)

    if isinstance(data, list):
        items = data
    else:
        items = data.get("data", [])

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
    return added


@asynccontextmanager
async def lifespan(app: FastAPI):
    create_tables()
    seed_providers()
    print("[startup] Providers seeded")

    with Session(engine) as session:
        # Wipe all stale models from old seed — only keep what sync fetches
        for m in session.exec(select(Model)).all():
            session.delete(m)
        session.commit()

        or_added = await sync_openrouter_models(session)
        if or_added:
            print(f"[startup] Synced {or_added} OpenRouter models")
        oc_added = await sync_opencode_models(session)
        if oc_added:
            print(f"[startup] Synced {oc_added} OpenCode models")
        session.commit()
    yield

app = FastAPI(title="NightCode Backend", version="1.0.0", lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=os.getenv("CORS_ORIGINS", "http://localhost:3000").split(","),
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(chat_router)
app.include_router(providers_router)
app.include_router(conversations_router)
app.include_router(settings_router)


@app.get("/api/health")
def health():
    return {"status": "ok", "version": "1.0.0"}
