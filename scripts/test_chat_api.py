import os

from openai import OpenAI


DEFAULT_MODEL = "local-llama"
MODEL_ENV_VAR = "GATEWAY_SMOKE_MODEL"


def main() -> None:
    model = os.getenv(MODEL_ENV_VAR, DEFAULT_MODEL)

    client = OpenAI(
        api_key="sk-local-demo",
        base_url="http://localhost:8080/v1",
    )

    response = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "user", "content": "Explain LLM gateways."},
        ],
    )

    print(response.choices[0].message.content)


if __name__ == "__main__":
    main()
