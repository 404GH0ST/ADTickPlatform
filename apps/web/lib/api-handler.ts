import { NextResponse } from "next/server";

export async function handleAdminIdRoute<T>(
  context: { params: Promise<Record<string, string>> },
  idParamName: string,
  idName: string,
  actionName: string,
  actionFn: (id: number) => Promise<T>,
) {
  const params = await context.params;
  const idStr = params[idParamName];
  if (!idStr) {
    return NextResponse.json(
      { status: "failed", message: `${idName} is invalid.` },
      { status: 400 },
    );
  }

  const id = Number(idStr);
  if (!Number.isInteger(id) || id <= 0) {
    return NextResponse.json(
      { status: "failed", message: `${idName} is invalid.` },
      { status: 400 },
    );
  }

  try {
    const data = await actionFn(id);
    return NextResponse.json({ status: "success", data });
  } catch (error) {
    const message = error instanceof Error ? error.message : `${actionName} failed`;
    return NextResponse.json({ status: "failed", message }, { status: 502 });
  }
}
