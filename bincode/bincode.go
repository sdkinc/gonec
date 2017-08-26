package bincode

import (
	"strings"

	"github.com/covrom/gonec/ast"
)

///////////////////////////////////////////////////////////////
// компиляция в байткод
///////////////////////////////////////////////////////////////

func BinaryCode(inast []ast.Stmt, reg int, lid *int) (bins BinCode) {
	for _, st := range inast {
		// перебираем все подвыражения и команды, и выстраиваем их в линию
		// если в команде есть выражение - определяем новый id регистра, присваиваем ему выражение, а в команду передаем id этого регистра
		switch s := st.(type) {
		case *ast.ExprStmt:
			bins = append(bins, addBinExpr(s.Expr, reg, lid)...)
		case *ast.IfStmt:
			*lid++
			lend := *lid
			// Если
			bins = append(bins, addBinExpr(s.If, reg, lid)...)
			*lid++
			lf := *lid
			bins = appendBin(bins,
				&BinJFALSE{
					Reg:    reg,
					JumpTo: lf,
				}, s)
			// Тогда
			bins = append(bins, BinaryCode(s.Then, reg, lid)...)
			bins = appendBin(bins,
				&BinJMP{
					JumpTo: lend,
				}, s)
			// ИначеЕсли
			bins = appendBin(bins,
				&BinLABEL{
					Label: lf,
				}, s)

			for _, elif := range s.ElseIf {
				stmtif := elif.(*ast.IfStmt)
				bins = append(bins, addBinExpr(stmtif.If, reg, lid)...)
				// если ложь, то перейдем на следующее условие
				*lid++
				li := *lid
				bins = appendBin(bins,
					&BinJFALSE{
						Reg:    reg,
						JumpTo: li,
					}, stmtif)
				bins = append(bins, BinaryCode(stmtif.Then, reg, lid)...)
				bins = appendBin(bins,
					&BinJMP{
						JumpTo: lend,
					}, stmtif)
				bins = appendBin(bins,
					&BinLABEL{
						Label: li,
					}, stmtif)
			}
			// Иначе
			if len(s.Else) > 0 {
				bins = append(bins, BinaryCode(s.Else, reg, lid)...)
			}
			// КонецЕсли
			bins = appendBin(bins,
				&BinLABEL{
					Label: lend,
				}, s)
		case *ast.TryStmt:
			// эта инструкция сообщает, в каком регистре будет отслеживаться ошибка выполнения кода до блока CATCH
			// по-умолчанию, ошибка в регистрах не отслеживается, а передается по уровням исполнения вирт. машины
			bins = appendBin(bins,
				&BinTRY{
					Reg: reg,
				}, s)
			bins = append(bins, BinaryCode(s.Try, reg+1, lid)...) // чтобы не затереть регистр с ошибкой, увеличиваем номер
			// CATCH работает как JFALSE, и определяет функцию ОписаниеОшибки()
			*lid++
			lend := *lid
			bins = appendBin(bins,
				&BinCATCH{
					Reg:    reg,
					JumpTo: lend,
				}, s)
			// тело обработки ошибки
			bins = append(bins, BinaryCode(s.Catch, reg, lid)...) // регистр с ошибкой больше не нужен, текст определен функцией
			// КонецПопытки
			bins = appendBin(bins,
				&BinLABEL{
					Label: lend,
				}, s)
			// снимаем со стека состояние обработки ошибок, чтобы последующий код не был включен в текущую обработку
			bins = appendBin(bins,
				&BinPOPTRY{
					Reg: reg,
				}, s)

		case *ast.ForStmt:
			// для каждого
			bins = append(bins, addBinExpr(s.Value, reg, lid)...)

			*lid++
			lend := *lid

			regiter := reg + 1
			regval := reg + 2
			regsub := reg + 3
			// инициализируем итератор, параметры цикла и цикл в стеке циклов
			bins = appendBin(bins,
				&BinFOREACH{
					Reg:        reg,
					RegIter:    regiter,
					BreakLabel: lend,
				}, s)
			*lid++
			li := *lid
			// очередная итерация
			// сюда же переходим по Продолжить
			bins = appendBin(bins,
				&BinLABEL{
					Label: li,
				}, s)
			bins = appendBin(bins,
				&BinNEXT{
					Reg:     reg,
					RegIter: regiter,
					RegVal:  regval,
					JumpTo:  lend,
				}, s)
			// устанавливаем переменную-итератор
			bins = appendBin(bins,
				&BinSET{
					Reg: regval,
					Id:  s.Var,
				}, s)

			bins = append(bins, BinaryCode(s.Stmts, regsub, lid)...)

			// повторяем итерацию
			bins = appendBin(bins,
				&BinJMP{
					JumpTo: li,
				}, s)

			// КонецЦикла
			bins = appendBin(bins,
				&BinLABEL{
					Label: lend,
				}, s)
			// снимаем со стека наличие цикла для Прервать и Продолжить
			bins = appendBin(bins,
				&BinPOPFOR{
					Reg: reg,
				}, s)

		case *ast.NumForStmt:
			// для .. по ..
			regfrom := reg + 1
			regto := reg + 2
			regsub := reg + 3

			bins = append(bins, addBinExpr(s.Expr1, regfrom, lid)...)
			bins = append(bins, addBinExpr(s.Expr2, regto, lid)...)

			*lid++
			lend := *lid

			// инициализируем итератор, параметры цикла и цикл в стеке циклов
			bins = appendBin(bins,
				&BinFORNUM{
					Reg:        reg,
					RegFrom:    regfrom,
					RegTo:      regto,
					BreakLabel: lend,
				}, s)
			*lid++
			li := *lid
			// очередная итерация
			// сюда же переходим по Продолжить
			bins = appendBin(bins,
				&BinLABEL{
					Label: li,
				}, s)
			bins = appendBin(bins,
				&BinNEXTNUM{
					Reg:    reg,
					JumpTo: lend, // сюда же переходим по Прервать
				}, s)
			// устанавливаем переменную-итератор
			bins = appendBin(bins,
				&BinSET{
					Reg: reg,
					Id:  s.Name,
				}, s)

			bins = append(bins, BinaryCode(s.Stmts, regsub, lid)...)

			// повторяем итерацию
			bins = appendBin(bins,
				&BinJMP{
					JumpTo: li,
				}, s)

			// КонецЦикла
			bins = appendBin(bins,
				&BinLABEL{
					Label: lend,
				}, s)
			// снимаем со стека наличие цикла для Прервать и Продолжить
			bins = appendBin(bins,
				&BinPOPFOR{
					Reg: reg,
				}, s)

		case *ast.LoopStmt:
			*lid++
			lend := *lid
			*lid++
			li := *lid
			bins = appendBin(bins,
				&BinWHILE{
					Reg:        reg,
					BreakLabel: lend,
				}, s)
			// очередная итерация
			// сюда же переходим по Продолжить
			bins = appendBin(bins,
				&BinLABEL{
					Label: li,
				}, s)
			bins = append(bins, addBinExpr(s.Expr, reg, lid)...)
			bins = appendBin(bins,
				&BinJFALSE{
					Reg:    reg,
					JumpTo: lend,
				}, s)
			// тело цикла
			bins = append(bins, BinaryCode(s.Stmts, reg+1, lid)...)

			// повторяем итерацию
			bins = appendBin(bins,
				&BinJMP{
					JumpTo: li,
				}, s)

			// КонецЦикла
			bins = appendBin(bins,
				&BinLABEL{
					Label: lend,
				}, s)
			// снимаем со стека наличие цикла для Прервать и Продолжить
			bins = appendBin(bins,
				&BinPOPFOR{
					Reg: reg,
				}, s)

		case *ast.BreakStmt:
			bins = appendBin(bins,
				&BinBREAK{}, s)

		case *ast.ContinueStmt:
			bins = appendBin(bins,
				&BinCONTINUE{}, s)

		case *ast.ReturnStmt:
			if len(s.Exprs) == 0 {
				bins = appendBin(bins,
					&BinLOAD{
						Reg: reg, // основной регистр
						Val: nil,
					}, s)
			}
			if len(s.Exprs) == 1 {
				// одиночное значение в reg
				bins = append(bins, addBinExpr(s.Exprs[0], reg, lid)...)
			} else {
				// создание слайса в reg
				bins = appendBin(bins,
					&BinMAKESLICE{
						Reg: reg,
						Len: len(s.Exprs),
						Cap: len(s.Exprs),
					}, s)

				for i, ee := range s.Exprs {
					bins = append(bins, addBinExpr(ee, reg+1, lid)...)
					bins = appendBin(bins,
						&BinSETIDX{
							Reg:    reg,
							Index:  i,
							ValReg: reg + 1,
						}, ee)
				}
			}
			// в reg имеем значение или структуру возврата
			bins = appendBin(bins,
				&BinRET{
					Reg: reg,
				}, s)

		case *ast.ThrowStmt:
			bins = append(bins, addBinExpr(s.Expr, reg, lid)...)
			bins = appendBin(bins,
				&BinTHROW{
					Reg: reg,
				}, s)

		case *ast.ModuleStmt:
			bins = appendBin(bins,
				&BinMODULE{
					Name: s.Name,
					Code: BinaryCode(s.Stmts, 0, lid),
				}, s)
		case *ast.SwitchStmt:
			bins = append(bins, addBinExpr(s.Expr, reg, lid)...)
			// сравниваем с каждым case
			*lid++
			lend := *lid
			var default_stmt *ast.DefaultStmt
			for _, ss := range s.Cases {
				if ssd, ok := ss.(*ast.DefaultStmt); ok {
					default_stmt = ssd
					continue
				}
				*lid++
				li := *lid
				case_stmt := ss.(*ast.CaseStmt)
				bins = append(bins, addBinExpr(case_stmt.Expr, reg+1, lid)...)
				bins = appendBin(bins,
					&BinEQUAL{
						Reg:  reg + 2,
						Reg1: reg,
						Reg2: reg + 1,
					}, case_stmt)

				bins = appendBin(bins,
					&BinJFALSE{
						Reg:    reg + 2,
						JumpTo: li,
					}, case_stmt)
				bins = append(bins, BinaryCode(case_stmt.Stmts, reg, lid)...)
				bins = appendBin(bins,
					&BinJMP{
						JumpTo: lend,
					}, case_stmt)

				bins = appendBin(bins,
					&BinLABEL{
						Label: li,
					}, case_stmt)
			}
			if default_stmt != nil {
				bins = append(bins, BinaryCode(default_stmt.Stmts, reg, lid)...)
			}
			bins = appendBin(bins,
				&BinLABEL{
					Label: lend,
				}, s)

		case *ast.SelectStmt:
			*lid++
			lstart := *lid
			bins = appendBin(bins,
				&BinLABEL{
					Label: lstart,
				}, s)

			*lid++
			lend := *lid
			var default_stmt *ast.DefaultStmt
			for _, ss := range s.Cases {
				if ssd, ok := ss.(*ast.DefaultStmt); ok {
					default_stmt = ssd
					continue
				}
				*lid++
				li := *lid
				case_stmt := ss.(*ast.CaseStmt)
				e, ok := case_stmt.Expr.(*ast.ChanExpr)
				if !ok {
					bins = appendBin(bins,
						&BinERROR{
							Error: "При выборе вариантов из каналов допустимы только выражения с каналами",
						}, case_stmt)
					continue
				}
				// определяем значение справа
				bins = append(bins, addBinExpr(e.Rhs, reg, lid)...)
				if e.Lhs == nil {
					// слева нет значения - это временное чтение из канала без сохранения значения в переменной
					bins = appendBin(bins,
						&BinTRYRECV{
							Reg:    reg,
							RegVal: reg + 1,
							RegOk:  reg + 2,
						}, e.Rhs)
					bins = appendBin(bins,
						&BinJFALSE{
							Reg:    reg + 2,
							JumpTo: li,
						}, s)
				} else {
					// значение слева
					bins = append(bins, addBinExpr(e.Lhs, reg+1, lid)...)
					// слева канал - пишем в него правое
					bins = appendBin(bins,
						&BinTRYSEND{
							Reg:    reg + 1,
							RegVal: reg,
							RegOk:  reg + 2,
						}, e.Lhs)

					*lid++
					li2 := *lid

					bins = appendBin(bins,
						&BinJTRUE{
							Reg:    reg + 2,
							JumpTo: li2,
						}, s)

					// иначе справа канал, а слева переменная (установим, если прочитали из канала)
					bins = appendBin(bins,
						&BinTRYRECV{
							Reg:    reg,
							RegVal: reg + 1,
							RegOk:  reg + 2,
						}, e.Rhs)
					bins = appendBin(bins,
						&BinJFALSE{
							Reg:    reg + 2,
							JumpTo: li,
						}, s)

					bins = append(bins, addBinLetExpr(e.Lhs, reg+1, lid)...)

					bins = appendBin(bins,
						&BinLABEL{
							Label: li2,
						}, s)

				}
				// отправили или прочитали - выполняем ветку кода и выходим из цикла
				bins = append(bins, BinaryCode(case_stmt.Stmts, reg, lid)...)

				// выходим из цикла
				bins = appendBin(bins,
					&BinJMP{
						JumpTo: lend,
					}, case_stmt)

				// к следующему case
				bins = appendBin(bins,
					&BinLABEL{
						Label: li,
					}, s)
			}
			// если ни одна из веток не сработала - проверяем default
			if default_stmt != nil {
				bins = append(bins, BinaryCode(default_stmt.Stmts, reg, lid)...)
			} else {
				// допускаем обработку других горутин
				bins = appendBin(bins,
					&BinGOSHED{}, s)
				bins = appendBin(bins,
					&BinJMP{
						JumpTo: lstart,
					}, s)
			}
			bins = appendBin(bins,
				&BinLABEL{
					Label: lend,
				}, s)

		case *ast.LetsStmt:

		case *ast.VarStmt:

		}
	}
	return
}

func appendBin(bins BinCode, b BinStmt, e ast.Pos) BinCode {
	b.SetPosition(e.Position())
	return append(bins, b)
}

func addBinLetExpr(e ast.Expr, reg int, lid *int) (bins BinCode) {
	// присваиваем значению переменной из e значение из регистра reg
	switch ee := e.(type) {
	case *ast.IdentExpr:
		bins = appendBin(bins,
			&BinSET{
				Reg: reg,
				Id:  ee.Id,
			}, e)

	case *ast.MemberExpr:

	case *ast.ItemExpr:

	case *ast.SliceExpr:

	default:
		// ошибка

	}
	return
}

func addBinExpr(expr ast.Expr, reg int, lid *int) (bins BinCode) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.NativeExpr:
		// добавляем команду загрузки значения
		bins = appendBin(bins,
			&BinLOAD{
				Reg: reg, // основной регистр
				Val: e.Value.Interface(),
			}, e)
	case *ast.NumberExpr:
		// команда на загрузку строки в регистр и ее преобразование в число, в регистр
		bins = appendBin(bins,
			&BinLOAD{
				Reg: reg,
				Val: e.Lit,
			}, e)

		bins = appendBin(bins,
			&BinCASTNUM{
				Reg: reg,
			}, e)
	case *ast.StringExpr:
		bins = appendBin(bins,
			&BinLOAD{
				Reg: reg,
				Val: e.Lit,
			}, e)
	case *ast.ConstExpr:
		b := BinLOAD{
			Reg: reg,
		}
		switch strings.ToLower(e.Value) {
		case "истина":
			b.Val = true
		case "ложь":
			b.Val = false
		case "null":
			b.Val = ast.NullVar

		default:
			b.Val = nil

		}
		bins = appendBin(bins, &b, e)
	case *ast.ArrayExpr:
		// создание слайса
		bins = appendBin(bins,
			&BinMAKESLICE{
				Reg: reg,
				Len: len(e.Exprs),
				Cap: len(e.Exprs),
			}, e)

		for i, ee := range e.Exprs {
			// каждое выражение сохраняем в следующем по номеру регистре (относительно регистра слайса)
			bins = append(bins, addBinExpr(ee, reg+1, lid)...)
			bins = appendBin(bins,
				&BinSETIDX{
					Reg:    reg,
					Index:  i,
					ValReg: reg + 1,
				}, ee)
		}
	case *ast.MapExpr:
		// создание мапы
		bins = appendBin(bins,
			&BinMAKEMAP{
				Reg: reg,
				Len: len(e.MapExpr),
			}, e)

		for k, ee := range e.MapExpr {
			bins = append(bins, addBinExpr(ee, reg+1, lid)...)
			bins = appendBin(bins,
				&BinSETKEY{
					Reg:    reg,
					Key:    k,
					ValReg: reg + 1,
				}, ee)
		}
	case *ast.IdentExpr:
		bins = appendBin(bins,
			&BinGET{
				Reg:    reg,
				Id:     e.Id,
				Dotted: strings.Contains(e.Lit, "."),
			}, e)
	case *ast.UnaryExpr:
		bins = append(bins, addBinExpr(e.Expr, reg, lid)...)
		bins = appendBin(bins,
			&BinUNARY{
				Reg: reg,
				Op:  rune(e.Operator[0]),
			}, e)
	case *ast.AddrExpr:
		bins = append(bins, addBinExpr(e.Expr, reg, lid)...)
		bins = appendBin(bins,
			&BinADDR{
				Reg: reg,
			}, e)
	case *ast.DerefExpr:
		bins = append(bins, addBinExpr(e.Expr, reg, lid)...)
		bins = appendBin(bins,
			&BinUNREF{
				Reg: reg,
			}, e)
	case *ast.ParenExpr:
		bins = append(bins, addBinExpr(e.SubExpr, reg, lid)...)
	case *ast.BinOpExpr:
		oper := OperMap[e.Operator]
		// сначала вычисляем левую часть
		bins = append(bins, addBinExpr(e.Lhs, reg, lid)...)
		switch oper {
		case LOR:
			*lid++
			lab := *lid
			// вставляем проверку на истину слева и возвращаем ее, не вычисляя правую часть, иначе возвращаем правую часть
			bins = appendBin(bins,
				&BinJTRUE{
					Reg:    reg,
					JumpTo: lab,
				}, e)
			bins = append(bins, addBinExpr(e.Rhs, reg, lid)...)
			bins = appendBin(bins,
				&BinLABEL{
					Label: lab,
				}, e)
		case LAND:
			*lid++
			lab := *lid
			// вставляем проверку на ложь слева и возвращаем ее, не вычисляя правую часть, иначе возвращаем правую часть
			bins = appendBin(bins,
				&BinJFALSE{
					Reg:    reg,
					JumpTo: lab,
				}, e)
			bins = append(bins, addBinExpr(e.Rhs, reg, lid)...)
			bins = appendBin(bins,
				&BinLABEL{
					Label: lab,
				}, e)
		default:
			bins = append(bins, addBinExpr(e.Rhs, reg+1, lid)...)
			bins = appendBin(bins,
				&BinOPER{
					RegL: reg, // сюда же помещается результат
					RegR: reg + 1,
					Op:   oper,
				}, e)
		}
	case *ast.TernaryOpExpr:
		bins = append(bins, addBinExpr(e.Expr, reg, lid)...)
		*lid++
		lab := *lid
		bins = appendBin(bins,
			&BinJFALSE{
				Reg:    reg,
				JumpTo: lab,
			}, e)
		// если истина - берем левое выражение
		bins = append(bins, addBinExpr(e.Lhs, reg, lid)...)
		// прыгаем в конец
		*lid++
		lend := *lid
		bins = appendBin(bins,
			&BinJMP{
				JumpTo: lend,
			}, e)

		// правое выражение
		bins = appendBin(bins,
			&BinLABEL{
				Label: lab,
			}, e)
		bins = append(bins, addBinExpr(e.Rhs, reg, lid)...)
		bins = appendBin(bins,
			&BinLABEL{
				Label: lend,
			}, e)

	case *ast.CallExpr:
		// если это анонимный вызов, то в reg сама функция, значит, параметры записываем в reg+1, иначе в reg
		var regoff int
		if e.Name == 0 {
			regoff = 1
		}

		// помещаем аргументы
		// либо в серию регистров, начиная с reg, если их <=7
		// либо в массив аргументов в reg
		if len(e.SubExprs) <= 7 {
			for i := 0; i < len(e.SubExprs); i++ {
				bins = append(bins, addBinExpr(e.SubExprs[i], reg+i+regoff, lid)...)
			}
		} else {
			bins = appendBin(bins,
				&BinMAKESLICE{
					Reg: reg + regoff,
					Len: len(e.SubExprs),
					Cap: len(e.SubExprs),
				}, e)

			for i, ee := range e.SubExprs {
				// каждое выражение сохраняем в следующем по номеру регистре (относительно регистра слайса)
				bins = append(bins, addBinExpr(ee, reg+1+regoff, lid)...)
				bins = appendBin(bins,
					&BinSETIDX{
						Reg:    reg + regoff,
						Index:  i,
						ValReg: reg + 1,
					}, ee)
			}
		}
		bins = appendBin(bins,
			&BinCALL{
				Name:    e.Name,
				NumArgs: len(e.SubExprs),
				RegArgs: reg, // для анонимных (Name==0) - тут будет функция, иначе первый аргумент (см. выше)
				VarArg:  e.VarArg,
				Go:      e.Go,
			}, e)

	case *ast.AnonCallExpr:
		// помещаем в регистр значение функции (тип func, или ссылку на него, или интерфейс с ним)
		bins = append(bins, addBinExpr(e.Expr, reg, lid)...)
		// далее аргументы, как при вызове обычной функции
		bins = append(bins, addBinExpr(&ast.CallExpr{
			Name:     0,
			SubExprs: e.SubExprs,
			VarArg:   e.VarArg,
			Go:       e.Go,
		}, reg, lid)...) // передаем именно reg, т.к. он для Name==0 означает функцию, которую надо вызвать в BinCALL

	case *ast.MemberExpr:
		// здесь идет только вычисление значения свойства
		bins = append(bins, addBinExpr(e.Expr, reg, lid)...)
		bins = appendBin(bins,
			&BinGETMEMBER{
				Name: e.Name,
				Reg:  reg,
			}, e)
	case *ast.ItemExpr:
		// только вычисление значения по индексу
		bins = append(bins, addBinExpr(e.Value, reg, lid)...)
		bins = append(bins, addBinExpr(e.Index, reg+1, lid)...)
		bins = appendBin(bins,
			&BinGETIDX{
				Reg:      reg,
				RegIndex: reg + 1,
			}, e)
	case *ast.SliceExpr:
		// только вычисление субслайса
		bins = append(bins, addBinExpr(e.Value, reg, lid)...)
		bins = append(bins, addBinExpr(e.Begin, reg+1, lid)...)
		bins = append(bins, addBinExpr(e.End, reg+2, lid)...)
		bins = appendBin(bins,
			&BinGETSUBSLICE{
				Reg:      reg,
				BeginReg: reg + 1,
				EndReg:   reg + 2,
			}, e)
	case *ast.FuncExpr:
		*lid++
		lend := *lid
		bins = appendBin(bins,
			&BinFUNC{
				Reg:      reg,
				Name:     e.Name,
				Code:     BinaryCode(e.Stmts, 0, lid),
				Args:     e.Args,
				VarArg:   e.VarArg,
				ReturnTo: lend,
			}, e)
		// КонецФункции
		bins = appendBin(bins,
			&BinLABEL{
				Label: lend,
			}, e)
		// возвращаем значения в регистре reg, установленные функцией
		bins = appendBin(bins,
			&BinRET{
				Reg: reg,
			}, e)

	case *ast.LetExpr:
		// пока не используется (не распознается парсером), планируется добавить предопределенные значения для функций
	case *ast.TypeCast:
		bins = append(bins, addBinExpr(e.CastExpr, reg, lid)...)
		if e.TypeExpr == nil {
			bins = appendBin(bins,
				&BinLOAD{
					Reg: reg + 1,
					Val: e.Type,
				}, e)
		} else {
			bins = append(bins, addBinExpr(e.TypeExpr, reg+1, lid)...)
			bins = appendBin(bins,
				&BinSETNAME{
					Reg: reg + 1,
				}, e)
		}
		bins = appendBin(bins,
			&BinCASTTYPE{
				Reg:     reg,
				TypeReg: reg + 1,
			}, e)
	case *ast.MakeExpr:
		if e.TypeExpr == nil {
			bins = appendBin(bins,
				&BinLOAD{
					Reg: reg,
					Val: e.Type,
				}, e)
		} else {
			bins = append(bins, addBinExpr(e.TypeExpr, reg, lid)...)
			bins = appendBin(bins,
				&BinSETNAME{
					Reg: reg,
				}, e)
		}
		bins = appendBin(bins,
			&BinMAKE{
				Reg: reg,
			}, e)
	case *ast.MakeChanExpr:
		if e.SizeExpr == nil {
			bins = appendBin(bins,
				&BinLOAD{
					Reg: reg,
					Val: int(0),
				}, e)
		} else {
			bins = append(bins, addBinExpr(e.SizeExpr, reg, lid)...)
		}
		bins = appendBin(bins,
			&BinMAKECHAN{
				Reg: reg,
			}, e)
	case *ast.MakeArrayExpr:
		bins = append(bins, addBinExpr(e.LenExpr, reg, lid)...)
		if e.CapExpr == nil {
			bins = appendBin(bins,
				&BinMV{
					RegFrom: reg,
					RegTo:   reg + 1,
				}, e)
		} else {
			bins = append(bins, addBinExpr(e.CapExpr, reg+1, lid)...)
		}
		bins = appendBin(bins,
			&BinMAKEARR{
				Reg:    reg,
				RegCap: reg + 1,
			}, e)
	case *ast.ChanExpr:
		// TODO: тут все зависит от операндов слева и справа, канал там, или переменная, будут условные переходы и присвоение
		// возвращает в reg+1 булево значение - прочитано/записано, или нет
		// есть тип CHANSEND / RECV

	case *ast.AssocExpr:
		// TODO: тут будет присвоение

	}

	return
}