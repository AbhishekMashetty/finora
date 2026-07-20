// Shared API/domain types. Kept minimal for Phase 0 — only the auth + user
// shapes actually consumed by the UI so far. Other domains (accounts,
// transactions, budgets, goals, reports, notifications) will get their own
// types when their real pages are built in a later phase.

export interface User {
  id: string;
  email: string;
  name: string;
  [key: string]: unknown;
}

export interface Settings {
  user_id: string;
  currency: string;
  timezone: string;
  updated_at: string;
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  user: User;
}

export interface RefreshResponse {
  access_token: string;
  refresh_token: string;
}

export interface RegisterResponse {
  user: User;
}

// expense-service domains (accounts, transactions, categories)

export interface Account {
  id: string;
  user_id: string;
  name: string;
  type: string;
  currency: string;
  balance: number;
  created_at: string;
  updated_at: string;
}

export interface Category {
  id: string;
  user_id: string;
  name: string;
  type: "income" | "expense";
  created_at: string;
  updated_at: string;
}

export interface Transaction {
  id: string;
  user_id: string;
  account_id: string;
  category_id: string | null;
  type: "income" | "expense";
  amount: number;
  currency: string;
  date: string;
  note?: string;
  created_at: string;
  updated_at: string;
}

export interface ImportRowError {
  row: number;
  message: string;
}

export interface ImportResult {
  imported: number;
  skipped: number;
  errors: ImportRowError[];
}

// budget-service domains (budgets, goals, reports)

export interface Budget {
  id: string;
  user_id: string;
  category: string;
  amount: number;
  period: "weekly" | "monthly" | "yearly";
  created_at: string;
  updated_at: string;
}

export interface Goal {
  id: string;
  user_id: string;
  name: string;
  target_amount: number;
  target_date: string;
  current_amount: number;
  created_at: string;
  updated_at: string;
}

export interface CategorySummary {
  category: string;
  period: string;
  budgeted: number;
  actual: number;
  remaining: number;
}

export interface ReportSummary {
  from: string;
  to: string;
  categories: CategorySummary[];
  total_budgeted: number;
  total_actual: number;
}

// notification-service domain

export interface Notification {
  id: string;
  user_id: string;
  title: string;
  message: string;
  type: string;
  read: boolean;
  created_at: string;
}
