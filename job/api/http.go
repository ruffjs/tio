package api

import (
	"context"
	"net/http"
	"strconv"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/pkg/errors"
	"ruff.io/tio/job"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
	rest "ruff.io/tio/pkg/restapi"
)

func Service(ctx context.Context, svc job.MgrService, wsTh *restful.WebService) *restful.WebService {
	tags := []string{"jobs"}

	wsJob := new(restful.WebService)
	wsJob.
		Path("/api/v1/jobs").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	jsl := job.StatusValues()
	tsl := job.TaskStatusValues()

	// for job

	wsJob.Route(wsJob.POST("/").
		To(createJobHandler(ctx, svc)).
		Operation("create").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Notes(`Operation can be :`+"\n"+
			`- Direct method, with prefix "`+job.SysOpDirectMethodPrefix+`" , like "`+job.SysOpDirectMethodPrefix+`turnOnLight"`+"\n"+
			`- Update shadow, with prefix "`+job.SysOpUpdateShadowPrefix+`" , like "`+job.SysOpUpdateShadowPrefix+`reportConfig"`+"\n"+
			`- Custom, with no "$" prefix, like "turnOnLight"`+"\n\n"+
			`Job doc:`+"\n"+
			`- When operation is kind of direct method, job doc should like: {"method":"xxx", "responseTimeout": 5, "data":{}}`+"\n"+
			`- When operation is kind of update shadow, job doc should like: {"state":{"desired": {"xxx":"yy"}}}`+"\n"+
			`- When operation custom, job doc can be any json object, eg: {"xxx": "yyy"}`,
		).
		Reads(job.CreateReq{}).
		Returns(200, "OK", rest.RespOK(job.Detail{})))
	wsJob.Route(wsJob.GET("/{jobId}").
		To(getJobHandler(ctx, svc)).
		Operation("get").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsJob.PathParameter("jobId", "")).
		Returns(200, "OK", rest.RespOK(job.Detail{})))
	wsJob.Route(wsJob.PATCH("/{jobId}").
		To(updateJobHandler(ctx, svc)).
		Operation("update").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsJob.PathParameter("jobId", "")).
		Reads(job.UpdateReq{}).
		Returns(200, "OK", rest.RespOK("")))
	wsJob.Route(wsJob.PUT("/{jobId}/cancel").
		To(cancelJobHandler(ctx, svc)).
		Operation("cancel").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsJob.PathParameter("jobId", "")).
		Param(wsJob.QueryParameter("force", "").DataType("boolean").DefaultValue("false")).
		Reads(job.CancelReq{}).
		Returns(200, "OK", rest.RespOK("")))
	wsJob.Route(wsJob.DELETE("/{jobId}").
		To(deleteJobHandler(ctx, svc)).
		Operation("delete").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsJob.PathParameter("jobId", "")).
		Param(wsJob.QueryParameter("force", "").DataType("boolean").DefaultValue("false")).
		Returns(200, "OK", rest.RespOK("")))
	wsJob.Route(wsJob.GET("/").
		To(queryJobHandler(ctx, svc)).
		Operation("query").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsJob.QueryParameter("status", "job status").PossibleValues(jsl)).
		Param(wsJob.QueryParameter("operation", "job operation").DataType("string")).
		Param(wsJob.QueryParameter("pageIndex", "page index, from 1").DataType("integer").DefaultValue("1")).
		Param(wsJob.QueryParameter("pageSize", "page size, from 1").DataType("integer").DefaultValue("10")).
		Returns(200, "OK", rest.RespOK(job.Page{})))
	wsJob.Route(wsJob.GET("/{jobId}/tasks").
		To(queryJobTaskHandler(ctx, svc)).
		Operation("query-job-tasks").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsTh.PathParameter("jobId", "")).
		Param(wsJob.QueryParameter("status", "task status").PossibleValues(tsl)).
		Param(wsJob.QueryParameter("operation", "job operation").DataType("string")).
		Param(wsJob.QueryParameter("pageIndex", "page index, from 1").DataType("integer").DefaultValue("1")).
		Param(wsJob.QueryParameter("pageSize", "page size, from 1").DataType("integer").DefaultValue("10")).
		Returns(200, "OK", rest.RespOK(job.TaskPage{})))

	//for task

	wsTh.Route(wsTh.PUT("/{thingId}/jobs/{jobId}/cancel").
		To(cancelTaskHandler(ctx, svc)).
		Operation("cancel-task").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsTh.PathParameter("thingId", "")).
		Param(wsTh.PathParameter("jobId", "")).
		Param(wsTh.QueryParameter("force", "").DataType("boolean").DefaultValue("false")).
		Reads(job.CancelTaskReq{}).
		Returns(200, "OK", rest.RespOK("")))
	wsTh.Route(wsTh.GET("/{thingId}/jobs/{jobId}/task/{taskId}").
		To(getTaskHandler(ctx, svc)).
		Operation("get-task").
		Doc("get detail of task").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsTh.PathParameter("thingId", "")).
		Param(wsTh.PathParameter("jobId", "")).
		Param(wsTh.PathParameter("taskId", "")).
		Returns(200, "OK", rest.RespOK(job.Task{})))
	wsTh.Route(wsTh.DELETE("/{thingId}/jobs/{jobId}/task/{taskId}").
		To(deleteTaskHandler(ctx, svc)).
		Operation("delete-task").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsTh.PathParameter("jobId", "")).
		Param(wsTh.PathParameter("thingId", "")).
		Param(wsTh.PathParameter("taskId", "")).
		Param(wsTh.QueryParameter("force", "").DataType("boolean").DefaultValue("false")).
		Returns(200, "OK", rest.RespOK("")))
	wsTh.Route(wsTh.GET("/{thingId}/tasks").
		To(queryThingTaskHandler(ctx, svc)).
		Operation("query-job-tasks").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(wsTh.PathParameter("thingId", "")).
		Param(wsTh.QueryParameter("status", "task status").PossibleValues(tsl)).
		Param(wsTh.QueryParameter("operation", "job operation").DataType("string")).
		Param(wsTh.QueryParameter("pageIndex", "page index, from 1").DataType("integer").DefaultValue("1")).
		Param(wsTh.QueryParameter("pageSize", "page size, from 1").DataType("integer").DefaultValue("10")).
		Returns(200, "OK", rest.RespOK(job.TaskPage{})))

	return wsJob
}

func createJobHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var cr job.CreateReq
		if err := r.ReadEntity(&cr); err != nil {
			log.Infof("Error decoding body for create job: %v", err)
			rest.SendResp(w, 400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}
		if j, err := svc.CreateJob(ctx, cr); err != nil {
			log.Errorf("Create job error, req=%#v, error: %v", cr, err)
			checkErrAndSend(err, w)
		} else {
			log.Infof("Create job success, jobId=%q", j.JobId)
			rest.SendRespOK(w, j)
		}
	}
}

func updateJobHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var req job.UpdateReq
		jobId := r.PathParameter("jobId")
		if err := r.ReadEntity(&req); err != nil {
			log.Infof("Error decoding body for update job: %v", err)
			rest.SendResp(w, 400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}
		if err := svc.UpdateJob(ctx, jobId, req); err != nil {
			log.Errorf("Create job error, jobId=%q, req=%#v, error: %v", jobId, req, err)
			checkErrAndSend(err, w)
		} else {
			log.Infof("Update job success, jobId=%q, req=%#v", jobId, req)
			rest.SendRespOK[any](w, nil)
		}
	}
}

func cancelJobHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var req job.CancelReq
		var force bool
		jobId := r.PathParameter("jobId")
		if f := r.QueryParameter("force"); f == "true" {
			force = true
		}

		if err := r.ReadEntity(&req); err != nil {
			log.Infof("Error decoding body for update job: %v", err)
			rest.SendResp(w, 400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}
		if err := svc.CancelJob(ctx, jobId, req, force); err != nil {
			log.Errorf("Cancel job error, jobId=%q, req=%#v, error: %v", jobId, req, err)
			checkErrAndSend(err, w)
		} else {
			log.Infof("Cancel job success, jobId=%q, req=%#v", jobId, req)
			rest.SendRespOK[any](w, nil)
		}
	}
}

func deleteJobHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var force bool
		jobId := r.PathParameter("jobId")
		if f := r.QueryParameter("force"); f == "true" {
			force = true
		}
		if _, err := svc.DeleteJob(ctx, jobId, force); err != nil {
			log.Errorf("Delete job error, jobId=%q, force=%v, error: %v", jobId, force, err)
			checkErrAndSend(err, w)
		} else {
			log.Infof("Delete job success, jobId=%q, force=%v", jobId, force)
			rest.SendRespOK[any](w, nil)
		}
	}
}

func cancelTaskHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var req job.CancelTaskReq
		var force bool
		jobId := r.PathParameter("jobId")
		thingId := r.PathParameter("thingId")
		if f := r.QueryParameter("force"); f == "true" {
			force = true
		}

		if err := r.ReadEntity(&req); err != nil {
			log.Infof("Error decoding body for cancel task: %v", err)
			rest.SendResp(w, 400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}
		if err := svc.CancelTask(ctx, thingId, jobId, req, force); err != nil {
			log.Errorf("Cancel task error, jobId=%q, req=%#v, error: %v", jobId, req, err)
			checkErrAndSend(err, w)
		} else {
			log.Infof("Cancel task success, jobId=%q, req=%#v", jobId, req)
			rest.SendRespOK[any](w, nil)
		}
	}
}

func deleteTaskHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var force bool
		var taskId int64
		jobId := r.PathParameter("jobId")
		thingId := r.PathParameter("thingId")

		if tid, err := strconv.ParseInt(r.PathParameter("taskId"), 10, 64); err != nil {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: "taskId: " + err.Error()})
			return
		} else {
			taskId = tid
		}
		if f := r.QueryParameter("force"); f == "true" {
			force = true
		}
		if _, err := svc.DeleteTask(ctx, thingId, jobId, taskId, force); err != nil {
			log.Errorf("Delete job error, jobId=%q, force=%v, error: %v", jobId, force, err)
			checkErrAndSend(err, w)
		} else {
			log.Infof("Delete job success, jobId=%q, force=%v", jobId, force)
			rest.SendRespOK[any](w, nil)
		}
	}
}

func getJobHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		jobId := r.PathParameter("jobId")
		if j, err := svc.GetJob(ctx, jobId); err != nil {
			log.Errorf("Get job error, jobId=%q, error: %v", jobId, err)
			checkErrAndSend(err, w)
		} else {
			rest.SendRespOK[any](w, j)
		}
	}
}

func getTaskHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var taskId int64
		thingId := r.PathParameter("thingId")
		jobId := r.PathParameter("jobId")

		if tid, err := strconv.ParseInt(r.PathParameter("taskId"), 10, 64); err != nil {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: "taskId: " + err.Error()})
			return
		} else {
			taskId = tid
		}
		if j, err := svc.GetTask(ctx, thingId, jobId, taskId); err != nil {
			log.Errorf("Get job error, jobId=%q, error: %v", jobId, err)
			checkErrAndSend(err, w)
		} else {
			rest.SendRespOK[any](w, j)
		}
	}
}

func queryJobHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var pq job.PageQuery
		if q, err := getPageQuery(r); err != nil {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: err.Error()})
			return
		} else {
			pq = q
		}
		if p, err := svc.QueryJob(ctx, pq); err != nil {
			log.Errorf("Query job error, query=%#v, error: %v", pq, err)
			checkErrAndSend(err, w)
		} else {
			rest.SendRespOK[any](w, p)
		}
	}
}

func queryJobTaskHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		jobId := r.PathParameter("jobId")
		var pq job.TaskPageQuery
		if q, err := getTaskPageQuery(r); err != nil {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: err.Error()})
			return
		} else {
			pq = q
		}
		if p, err := svc.QueryTaskForJob(ctx, jobId, pq); err != nil {
			log.Errorf("Query task for job error, query=%#v, error: %v", pq, err)
			checkErrAndSend(err, w)
		} else {
			rest.SendRespOK[any](w, p)
		}
	}
}

func queryThingTaskHandler(ctx context.Context, svc job.MgrService) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		//jobId := r.PathParameter("jobId")
		thingId := r.PathParameter("thingId")
		var pq job.TaskPageQuery
		if q, err := getTaskPageQuery(r); err != nil {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: err.Error()})
			return
		} else {
			pq = q
		}
		if p, err := svc.QueryTaskForThing(ctx, thingId, pq); err != nil {
			log.Errorf("Query task for thing error, query=%#v, error: %v", pq, err)
			checkErrAndSend(err, w)
		} else {
			rest.SendRespOK[any](w, p)
		}
	}
}

func checkErrAndSend(err error, w http.ResponseWriter) {
	var he model.HttpErr
	if ok := errors.As(err, &he); ok {
		rest.SendResp(w, he.HttpCode, rest.Resp[string]{Code: he.Code, Message: err.Error()})
	} else {
		rest.SendResp(w, 500, rest.Resp[string]{Code: 500, Message: err.Error()})
	}
}

func getTaskPageQuery(r *restful.Request) (q job.TaskPageQuery, err error) {
	s := r.QueryParameter("status")
	if s != "" {
		if q.Status, err = job.TaskStatusOf(s); err != nil {
			err = errors.WithMessagef(err, "status")
			return
		}
	}
	q.Operation = r.QueryParameter("operation")
	q.PageQuery.PageIndex, q.PageQuery.PageSize, err = getPageArgs(r)
	return
}

func getPageQuery(r *restful.Request) (q job.PageQuery, err error) {
	s := r.QueryParameter("status")
	if s != "" {
		if q.Status, err = job.StatusOf(s); err != nil {
			err = errors.WithMessagef(err, "status")
			return
		}
	}
	q.Operation = r.QueryParameter("operation")
	q.PageQuery.PageIndex, q.PageQuery.PageSize, err = getPageArgs(r)
	return
}

func getPageArgs(r *restful.Request) (pageIndex, pageSize int, err error) {
	if pageIndex, err = strconv.Atoi(r.QueryParameter("pageIndex")); err != nil {
		err = errors.WithMessagef(err, "pageIndex")
		return
	}
	if pageSize, err = strconv.Atoi(r.QueryParameter("pageSize")); err != nil {
		err = errors.WithMessage(err, "pageSize")
		return
	}
	return
}
