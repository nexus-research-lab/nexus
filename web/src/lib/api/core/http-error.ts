export class UnauthorizedError extends Error {
  constructor(message = "未登录或登录状态已过期") {
    super(message);
    this.name = "UnauthorizedError";
  }
}

export class ApiRequestError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiRequestError";
    this.status = status;
  }
}
